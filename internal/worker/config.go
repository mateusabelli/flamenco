package worker

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v2"
	"projects.blender.org/studio/flamenco/internal/appinfo"
	"projects.blender.org/studio/flamenco/pkg/website"
)

var (
	errURLWithoutHostName = errors.New("manager URL should contain a host name")
)

var (
	// config- and credentialsFilename are used to specify where flamenco will
	// write its config/credentials file. If the path is not absolute, it will
	// use the flamenco binary location as the relative root path. These are not
	// intended to be changed during runtime.
	credentialsFilename = "flamenco-worker-credentials.yaml"
	configFilename      = "flamenco-worker.yaml"
)

type configFileType string

var (
	configFileTypeConfiguration configFileType = "Configuration"
	configFileTypeCredentials   configFileType = "Credentials"
)

var defaultConfig = WorkerConfig{
	ConfiguredManager: "", // Auto-detect by default.
	TaskTypes:         []string{"blender", "ffmpeg", "file-management", "misc"},
}

// WorkerConfig represents the configuration of a single worker.
// It does not include authentication credentials.
type WorkerConfig struct {
	WorkerName string `yaml:"worker_name" json:"worker_name"`

	// ConfiguredManager is the Manager URL that's in the configuration file.
	ConfiguredManager string `yaml:"manager_url" json:"manager_url"`

	// ManagerURL is the Manager URL to use by the Worker. It could come from the
	// configuration file, but also from autodiscovery via UPnP/SSDP.
	ManagerURL string `yaml:"-" json:"-"`

	TaskTypes       []string `yaml:"task_types" json:"task_types"`
	RestartExitCode int      `yaml:"restart_exit_code" json:"restart_exit_code"`

	// LinuxOOMScoreAdjust controls the Linux out-of-memory killer. Is used when
	// spawning a sub-process, to adjust the likelyness that that subprocess is
	// killed rather than Flamenco Worker itself. That way Flamenco Worker can
	// report the failure to the Manager.
	//
	// If the Worker itself would be OOM-killed, it would just be restarted and
	// get the task it was already working on, causing an infinite OOM-loop.
	//
	// If this value is not specified in the configuration file, Flamenco Worker
	// will not attempt to adjust its OOM score.
	LinuxOOMScoreAdjust *int `yaml:"oom_score_adjust" json:"oom_score_adjust"`
}

type WorkerCredentials struct {
	WorkerID string `yaml:"worker_id" json:"worker_id"`
	Secret   string `yaml:"worker_secret" json:"worker_secret"`
}

// FileConfigWrangler is the default config wrangler that actually reads & writes files.
type FileConfigWrangler struct {
	// In-memory copy of the worker configuration.
	wc    *WorkerConfig
	creds *WorkerCredentials
}

// NewConfigWrangler returns ConfigWrangler that reads files.
func NewConfigWrangler() FileConfigWrangler {
	return FileConfigWrangler{}
}

type WorkerConfigPaths struct {
	Main        string
	Credentials string
}

// ConfigPaths returns the absolute file paths Flamenco Worker will use to load
// its configuration. If the path cannot be made absolute, an error will be
// logged and a relative path will be returned instead.
func (fcw *FileConfigWrangler) ConfigPaths() WorkerConfigPaths {
	var err error
	paths := WorkerConfigPaths{}

	// configFilename is used as-is.
	paths.Main, err = filepath.Abs(configFilename)
	if err != nil {
		log.Error().
			AnErr("cause", err).
			Str("filepath", configFilename).
			Msg("could not make the main configuration file path an absolute path")
		paths.Main = configFilename
	}

	// credentialsFilename is always looked up somewhere in the user's home dir.
	paths.Credentials, err = fcw.credentialsAbsPath()
	if err != nil {
		log.Error().
			AnErr("cause", err).
			Str("filepath", credentialsFilename).
			Msg("could not make the credentials configuration file path an absolute path")
		paths.Credentials = credentialsFilename
	}

	return paths
}

// WorkerConfig returns the worker configuration, or the default config if
// there is no config file. Configuration is only loaded from disk once;
// subsequent calls return the same config.
func (fcw *FileConfigWrangler) WorkerConfig() (WorkerConfig, error) {
	if fcw.wc != nil {
		return *fcw.wc, nil
	}

	wc := fcw.DefaultConfig()
	err := fcw.loadConfig(configFilename, &wc)

	if err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// The config file not existing is fine; just use the defaults.
		case errors.Is(err, io.EOF):
			// The config file exists but is empty; treat as non-existent.
		default:
			return wc, err
		}
	}

	fcw.wc = &wc

	man := strings.TrimSpace(wc.ConfiguredManager)
	if man != "" {
		fcw.SetManagerURL(man)
	}

	return wc, nil
}

func (fcw *FileConfigWrangler) SaveConfig() error {
	err := fcw.writeConfig(configFilename, configFileTypeConfiguration, fcw.wc)
	if err != nil {
		return fmt.Errorf("writing to %s: %w", configFilename, err)
	}
	return nil
}

func (fcw *FileConfigWrangler) WorkerCredentials() (WorkerCredentials, error) {
	filepath, err := appinfo.InFlamencoHome(credentialsFilename)
	if err != nil {
		return WorkerCredentials{}, err
	}

	var creds WorkerCredentials
	err = fcw.loadConfig(filepath, &creds)
	if err != nil {
		return WorkerCredentials{}, err
	}

	log.Info().
		Str("filename", filepath).
		Msg("loaded credentials")
	return creds, nil
}

func (fcw *FileConfigWrangler) SaveCredentials(creds WorkerCredentials) error {
	fcw.creds = &creds

	filepath, err := fcw.credentialsAbsPath()
	if err != nil {
		return err
	}

	err = fcw.writeConfig(filepath, configFileTypeCredentials, creds)
	if err != nil {
		return fmt.Errorf("writing to %s: %w", filepath, err)
	}
	return nil
}

func (fcw *FileConfigWrangler) credentialsAbsPath() (string, error) {
	filepath, err := appinfo.InFlamencoHome(credentialsFilename)
	return filepath, err
}

// SetManagerURL overwrites the Manager URL in the cached configuration.
// This is an in-memory change only, and will not be written to the config file.
func (fcw *FileConfigWrangler) SetManagerURL(managerURL string) {
	fcw.wc.ManagerURL = managerURL
}

func (fcw *FileConfigWrangler) SetRestartExitCode(code int) {
	fcw.wc.RestartExitCode = code
}

// DefaultConfig returns a fairly sane default configuration.
func (fcw FileConfigWrangler) DefaultConfig() WorkerConfig {
	return defaultConfig
}

// WriteConfig stores a struct as YAML file.
func (fcw FileConfigWrangler) writeConfig(filename string, filetype configFileType, config interface{}) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	tempFilename := filename + "~"
	f, err := os.OpenFile(tempFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	fmt.Fprintf(f, "# %s file for Flamenco Worker.\n", filetype)
	fmt.Fprintln(f, "#")
	switch filetype {
	case configFileTypeConfiguration:
		fmt.Fprintf(f, "# For an explanation of the fields, refer to %s\n", website.WorkerConfigURL)
	case configFileTypeCredentials:
		fmt.Fprintln(f, "# This file is not meant to be manually edited. Removing this file is fine, and")
		fmt.Fprintln(f, "# will cause the Worker to re-register as a new Worker.")
		fmt.Fprintln(f, "#")
		fmt.Fprintf(f, "# For more information, refer to %s\n", website.WorkerConfigURL)
	}
	fmt.Fprintln(f, "#")
	fmt.Fprintln(f, "# NOTE: this file may be overwritten by Flamenco Worker.")
	fmt.Fprintln(f, "#")
	now := time.Now()
	fmt.Fprintf(f, "# This file was written on %s\n\n", now.Format("2006-01-02 15:04:05 -07:00"))

	n, err := f.Write(data)
	if err != nil {
		f.Close() // ignore errors here
		return err
	}
	if n < len(data) {
		f.Close() // ignore errors here
		return io.ErrShortWrite
	}
	if err = f.Close(); err != nil {
		return err
	}

	log.Debug().Str("filename", tempFilename).Msg("config file written")
	log.Debug().
		Str("from", tempFilename).
		Str("to", filename).
		Msg("renaming config file")
	if err := os.Rename(tempFilename, filename); err != nil {
		return err
	}
	log.Info().Str("filename", filename).Msg("Saved configuration file")

	return nil
}

// LoadConfig loads a YAML configuration file into 'config'
func (fcw FileConfigWrangler) loadConfig(filename string, config interface{}) error {
	// Log which directory the config is loaded from.
	filepath, err := filepath.Abs(filename)
	if err != nil {
		log.Warn().Err(err).Str("filename", filename).
			Msg("config loader: unable to find absolute path of config file")
		log.Debug().Str("filename", filename).Msg("loading config file")
	} else {
		log.Debug().Str("path", filepath).Msg("loading config file")
	}

	f, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	if err = dec.Decode(config); err != nil {
		return err
	}

	return nil
}

// ParseURL allows URLs without scheme (assumes HTTP).
func ParseURL(rawURL string) (*url.URL, error) {
	var err error
	var parsedURL *url.URL

	parsedURL, err = url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	// url.Parse() is a bit weird when there is no scheme.
	if parsedURL.Host == "" && parsedURL.Path != "" {
		// This case happens when you just enter a hostname, like manager='thehost'
		parsedURL.Host = parsedURL.Path
		parsedURL.Path = "/"
	}
	if parsedURL.Host == "" && parsedURL.Scheme != "" && parsedURL.Opaque != "" {
		// This case happens when you just enter a hostname:port, like manager='thehost:8083'
		parsedURL.Host = parsedURL.Scheme + ":" + parsedURL.Opaque
		parsedURL.Opaque = ""
		parsedURL.Scheme = "http"
	}
	if parsedURL.Scheme == "" {
		parsedURL.Scheme = "http"
	}
	if parsedURL.Host == "" {
		return nil, errURLWithoutHostName
	}

	return parsedURL, nil
}
