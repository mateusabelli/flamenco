package worker

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"projects.blender.org/studio/flamenco/pkg/api"
)

// Generate the mock for the client interface.
//go:generate go tool mockgen -destination mocks/client.gen.go -package mocks projects.blender.org/studio/flamenco/internal/worker FlamencoClient

// FlamencoClient is a wrapper for api.ClientWithResponsesInterface so that locally mocks can be created.
type FlamencoClient interface {
	api.ClientWithResponsesInterface
}
