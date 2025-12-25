package api_impl

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"projects.blender.org/studio/flamenco/internal/manager/config"
	"projects.blender.org/studio/flamenco/internal/manager/persistence"
	"projects.blender.org/studio/flamenco/pkg/api"
)

//go:generate go tool mockgen -destination mocks/varrepl.gen.go -package mocks projects.blender.org/studio/flamenco/internal/manager/api_impl VariableReplacer
type VariableReplacer interface {
	NewVariableExpander(audience config.VariableAudience, platform config.VariablePlatform) *config.VariableExpander
	ResolveVariables(audience config.VariableAudience, platform config.VariablePlatform) map[string]config.ResolvedVariable
	NewVariableToValueConverter(audience config.VariableAudience, platform config.VariablePlatform) *config.ValueToVariableReplacer
}

// replaceTaskVariables performs variable replacement for worker tasks.
func replaceTaskVariables(replacer VariableReplacer, task api.AssignedTask, worker persistence.Worker) api.AssignedTask {
	varExpander := replacer.NewVariableExpander(
		config.VariableAudienceWorkers,
		config.VariablePlatform(worker.Platform),
	)

	for cmdIndex, cmd := range task.Commands {
		for key, value := range cmd.Parameters {
			switch v := value.(type) {
			case string:
				task.Commands[cmdIndex].Parameters[key] = varExpander.Expand(v)

			case []string:
				replaced := make([]string, len(v))
				for idx := range v {
					replaced[idx] = varExpander.Expand(v[idx])
				}
				task.Commands[cmdIndex].Parameters[key] = replaced

			case []interface{}:
				replaced := make([]interface{}, len(v))
				for idx := range v {
					switch itemValue := v[idx].(type) {
					case string:
						replaced[idx] = varExpander.Expand(itemValue)
					default:
						replaced[idx] = itemValue
					}
				}
				task.Commands[cmdIndex].Parameters[key] = replaced

			default:
				continue
			}
		}
	}

	return task
}

// replaceTwoWayVariables replaces values with their variables.
// For example, when there is a variable `render = /render/flamenco`, an output
// path `/render/flamenco/output.png` will be replaced with
// `{render}/output.png`
//
// NOTE: this updates the job in place.
func replaceTwoWayVariables(replacer VariableReplacer, job *api.SubmittedJob) {
	valueToVariable := replacer.NewVariableToValueConverter(
		config.VariableAudienceWorkers,
		config.VariablePlatform(job.SubmitterPlatform),
	)

	// Only replace variables in settings and metadata, not in other job fields.
	if job.Settings != nil {
		for settingKey, settingValue := range *job.Settings {
			stringValue, ok := settingValue.(string)
			if !ok {
				continue
			}
			newValue := valueToVariable.Replace(stringValue)
			(*job.Settings)[settingKey] = newValue
		}
	}
	if job.Metadata != nil {
		for metaKey, metaValue := range *job.Metadata {
			newValue := valueToVariable.Replace(metaValue)
			(*job.Metadata)[metaKey] = newValue
		}
	}
}
