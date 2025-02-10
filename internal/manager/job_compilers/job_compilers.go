// Package job_compilers contains functionality to convert a Flamenco job
// definition into concrete tasks and commands to execute by Workers.
package job_compilers

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/uuid"
	"projects.blender.org/studio/flamenco/pkg/api"
)

var ErrJobTypeUnknown = errors.New("job type unknown")
var ErrJobTypeBadEtag = errors.New("job type etag does not match")

// Service can load & run job compilers.
type Service struct {
	registry    *require.Registry // Goja module registry.
	timeService TimeService

	// mutex ensures only one job compiler runs at a time, and protects
	// 'registry' from race conditions.
	mutex *sync.Mutex
}

type Compiler struct {
	jobType  string
	program  *goja.Program // Compiled JavaScript file.
	filename string        // The filename of that JS file.
}

type VM struct {
	runtime     *goja.Runtime // Goja VM containing the job compiler script.
	compiler    Compiler      // Program loaded into this VM.
	jobTypeEtag string        // Etag for this particular job type.
}

// jobCompileFunc is a function that fills job.Tasks.
type jobCompileFunc func(job *AuthoredJob) error

// TimeService is a service that can tell the current time.
type TimeService interface {
	Now() time.Time
}

// New returns a job compiler service.
func New(ts TimeService) (*Service, error) {
	initFileLoader()

	service := Service{
		timeService: ts,
		mutex:       new(sync.Mutex),
	}

	staticFileLoader := func(path string) ([]byte, error) {
		content, err := loadFileFromAnyFS(path)
		if err == os.ErrNotExist {
			// The 'require' module uses this to try different variations of the path
			// in order to find it (without .js, with .js, etc.), so don't log any of
			// such errors.
			return nil, require.ModuleFileDoesNotExistError
		}
		return content, err
	}

	service.registry = require.NewRegistry(require.WithLoader(staticFileLoader))
	service.registry.RegisterNativeModule("author", AuthorModule)
	service.registry.RegisterNativeModule("path", PathModule)
	service.registry.RegisterNativeModule("process", ProcessModule)

	return &service, nil
}

func (s *Service) Compile(ctx context.Context, sj api.SubmittedJob) (*AuthoredJob, error) {
	vm, err := s.compilerVMForJobType(sj.Type)
	if err != nil {
		return nil, err
	}

	if err := vm.checkJobTypeEtag(sj); err != nil {
		return nil, err
	}

	// Create an AuthoredJob from this SubmittedJob.
	aj := AuthoredJob{
		JobID:    uuid.New(),
		Created:  s.timeService.Now(),
		Name:     sj.Name,
		JobType:  sj.Type,
		Priority: sj.Priority,
		Status:   api.JobStatusUnderConstruction,

		Settings: make(JobSettings),
		Metadata: make(JobMetadata),
	}
	if sj.Settings != nil {
		for key, value := range sj.Settings.AdditionalProperties {
			aj.Settings[key] = value
		}
	}
	if sj.Metadata != nil {
		for key, value := range sj.Metadata.AdditionalProperties {
			aj.Metadata[key] = value
		}
	}

	if sj.Storage != nil && sj.Storage.ShamanCheckoutId != nil {
		aj.Storage.ShamanCheckoutID = *sj.Storage.ShamanCheckoutId
	}

	if sj.WorkerTag != nil {
		aj.WorkerTagUUID = *sj.WorkerTag
	}

	compiler, err := vm.getCompileJob()
	if err != nil {
		return nil, err
	}
	if err := compiler(&aj); err != nil {
		return nil, err
	}

	log.Info().
		Int("num_tasks", len(aj.Tasks)).
		Str("name", aj.Name).
		Str("jobtype", aj.JobType).
		Str("job", aj.JobID).
		Msg("job compiled")

	return &aj, nil
}

// ListJobTypes returns the list of available job types.
func (s *Service) ListJobTypes() api.AvailableJobTypes {
	jobTypes := make([]api.AvailableJobType, 0)

	compilers := loadScripts()
	for typeName := range compilers {
		compiler, err := s.compilerVMForJobType(typeName)
		if err != nil {
			log.Warn().Err(err).Str("jobType", typeName).Msg("unable to determine job type settings")
			continue
		}

		jobType, err := compiler.getJobTypeInfo()
		if err != nil {
			log.Warn().Err(err).Str("jobType", typeName).Msg("unable to determine job type settings")
			continue
		}

		jobTypes = append(jobTypes, jobType)
	}

	sort.Slice(jobTypes, func(i, j int) bool { return jobTypes[i].Name < jobTypes[j].Name })

	return api.AvailableJobTypes{JobTypes: jobTypes}
}

// GetJobType returns information about the named job type.
// Returns ErrJobTypeUnknown when the name doesn't correspond with a known job type.
func (s *Service) GetJobType(typeName string) (api.AvailableJobType, error) {
	compiler, err := s.compilerVMForJobType(typeName)
	if err != nil {
		return api.AvailableJobType{}, err
	}
	return compiler.getJobTypeInfo()
}

func (vm *VM) getCompileJob() (jobCompileFunc, error) {
	compileJob, isCallable := goja.AssertFunction(vm.runtime.Get("compileJob"))
	if !isCallable {
		return nil, JobScriptIncompleteError{
			scriptFilename: vm.compiler.filename,
			message:        "does not define a compileJob(job) function",
		}
	}

	// TODO: wrap this in a nicer way.
	return func(job *AuthoredJob) error {
		_, err := compileJob(nil, vm.runtime.ToValue(job))
		return err
	}, nil
}

type JobScriptIncompleteError struct {
	wrappedErr     error
	scriptFilename string
	message        string
}

func (err JobScriptIncompleteError) Error() string {
	if err.wrappedErr == nil {
		return fmt.Sprintf("script (%s) %s", err.scriptFilename, err.message)
	}
	return fmt.Sprintf("script (%s) %s: %v", err.scriptFilename, err.message, err.wrappedErr)
}

func (err JobScriptIncompleteError) Unwrap() error {
	return err.wrappedErr
}

func (vm *VM) getJobTypeInfo() (api.AvailableJobType, error) {
	jtValue := vm.runtime.Get("JOB_TYPE")
	if jtValue == nil {
		return api.AvailableJobType{}, JobScriptIncompleteError{
			scriptFilename: vm.compiler.filename,
			message:        "does not define a JOB_TYPE object",
		}
	}

	var ajt api.AvailableJobType
	if err := vm.runtime.ExportTo(jtValue, &ajt); err != nil {
		return api.AvailableJobType{}, JobScriptIncompleteError{
			wrappedErr:     err,
			scriptFilename: vm.compiler.filename,
			message:        "does not define a proper JOB_TYPE object",
		}
	}

	ajt.Name = vm.compiler.jobType
	ajt.Etag = vm.jobTypeEtag
	return ajt, nil
}

// getEtag gets the job type etag hash.
func (vm *VM) getEtag() (string, error) {
	jobTypeInfo, err := vm.getJobTypeInfo()
	if err != nil {
		return "", err
	}

	// Convert to JSON, then compute the SHA256sum to get the Etag.
	asBytes, err := json.Marshal(&jobTypeInfo)
	if err != nil {
		return "", err
	}

	hasher := sha1.New()
	hasher.Write(asBytes)
	hashsum := hasher.Sum(nil)
	return fmt.Sprintf("%x", hashsum), nil
}

// updateEtag sets vm.jobTypeEtag based on the job type info it contains.
func (vm *VM) updateEtag() error {
	etag, err := vm.getEtag()
	if err != nil {
		return err
	}

	vm.jobTypeEtag = etag
	return nil
}

func (vm *VM) checkJobTypeEtag(sj api.SubmittedJob) error {
	if sj.TypeEtag == nil || *sj.TypeEtag == "" {
		return nil
	}

	if vm.jobTypeEtag != *sj.TypeEtag {
		return fmt.Errorf("%w: expecting %q, submitted job has %q",
			ErrJobTypeBadEtag, vm.jobTypeEtag, *sj.TypeEtag)
	}

	return nil
}
