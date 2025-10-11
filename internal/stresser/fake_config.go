package stresser

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"github.com/rs/zerolog/log"

	"projects.blender.org/studio/flamenco/internal/worker"
)

type FakeConfig struct {
	workerName string
	creds      worker.WorkerCredentials
}

func NewFakeConfig(workerName, workerID, workerSecret string) *FakeConfig {
	return &FakeConfig{
		workerName: workerName,
		creds: worker.WorkerCredentials{
			WorkerID: workerID,
			Secret:   workerSecret,
		},
	}
}

func (fc *FakeConfig) WorkerConfig() (worker.WorkerConfig, error) {
	config := worker.NewConfigWrangler().DefaultConfig()
	config.WorkerName = fc.workerName
	config.ManagerURL = "http://localhost:8080/"
	return config, nil
}

func (fc *FakeConfig) WorkerCredentials() (worker.WorkerCredentials, error) {
	return fc.creds, nil
}

func (fc *FakeConfig) SaveCredentials(creds worker.WorkerCredentials) error {
	log.Info().
		Str("workerID", creds.WorkerID).
		Str("workerSecret", creds.Secret).
		Msg("remember these credentials for next time")
	fc.creds = creds
	return nil
}
