package config

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/rs/zerolog/log"
)

// Service provides access to Flamenco Manager configuration.
type Service struct {
	config        Conf
	forceFirstRun bool
	filename      string
}

func NewService() *Service {
	return &Service{
		config:   DefaultConfig(),
		filename: configFilename,
	}
}

func (s *Service) ForceFirstRun() {
	s.forceFirstRun = true
}

func (s *Service) Replace(config Conf) error {
	if err := config.Overwrite(s.filename); err != nil {
		return err
	}

	// Replace the in-memory config with the new one
	config.processAfterLoading()
	s.config = config
	log.Info().Str("filename", s.filename).Msg("in-memory configuration file replaced")
	return nil
}

// Load parses the flamenco-manager.yaml file, and returns whether this is
// likely to be the first run or not.
func (s *Service) Load() (bool, error) {
	config, err := loadConf(s.filename)
	s.config = config

	switch {
	case errors.Is(err, fs.ErrNotExist):
		// No configuration means first run.
		return true, nil
	case err != nil:
		return false, fmt.Errorf("loading %s: %w", s.filename, err)
	}

	// No shared storage configured means first run.
	return s.forceFirstRun || config.SharedStoragePath == "", nil
}

// ConfigFilename returns the filename of the configuration file.
func (s *Service) ConfigFilename() string {
	return s.filename
}

func (s *Service) Get() *Conf {
	return &s.config
}

// Save writes the in-memory configuration to the config file.
func (s *Service) Save() error {
	err := s.config.Write(s.filename)
	if err != nil {
		return err
	}

	log.Info().Str("filename", s.filename).Msg("configuration file written")
	return nil
}

// Expose some functions of Conf here, for easier mocking of functionality via interfaces.
func (s *Service) NewVariableExpander(audience VariableAudience, platform VariablePlatform) *VariableExpander {
	return s.config.NewVariableExpander(audience, platform)
}
func (s *Service) NewVariableToValueConverter(audience VariableAudience, platform VariablePlatform) *ValueToVariableReplacer {
	return s.config.NewVariableToValueConverter(audience, platform)
}
func (s *Service) ResolveVariables(audience VariableAudience, platform VariablePlatform) map[string]ResolvedVariable {
	return s.config.ResolveVariables(audience, platform)
}
func (s *Service) EffectiveStoragePath() string {
	return s.config.EffectiveStoragePath()
}
