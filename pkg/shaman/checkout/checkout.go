package checkout

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/rs/zerolog"
	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/crosspath"
	"projects.blender.org/studio/flamenco/pkg/shaman/filestore"
)

var (
	ErrMissingFiles = errors.New("unknown files requested in checkout")

	validCheckoutRegexp = regexp.MustCompile(`^[^/?*:;{}\\][^?*:;{}]*$`)
)

// Checkout symlinks the requested files into the checkout directory.
// Returns the actually-used checkout directory, relative to the configured checkout root.
func (m *Manager) Checkout(ctx context.Context, inputCheckout api.ShamanCheckout) (string, error) {
	logger := (*zerolog.Ctx(ctx))

	checkout := sanitizePaths(inputCheckout)

	logger.Debug().
		Str("checkoutPath", checkout.CheckoutPath).
		Msg("shaman: user requested checkout creation")

	// Actually create the checkout.
	resolvedCheckoutInfo, err := m.PrepareCheckout(checkout.CheckoutPath)
	if err != nil {
		return "", err
	}
	logger = logger.With().Str("checkoutPath", resolvedCheckoutInfo.RelativePath).Logger()

	// The checkout directory was created, so if anything fails now, it should be erased.
	var checkoutOK bool
	defer func() {
		if !checkoutOK {
			err := m.EraseCheckout(checkout.CheckoutPath)
			if err != nil {
				logger.Error().Err(err).Msg("shaman: error erasing checkout directory")
			}
		}
	}()

	for _, fileSpec := range checkout.Files {
		blobPath, status := m.fileStore.ResolveFile(fileSpec.Sha, int64(fileSpec.Size), filestore.ResolveStoredOnly)
		if status != filestore.StatusStored {
			// Caller should upload this file before we can create the checkout.
			return "", ErrMissingFiles
		}

		if err := m.SymlinkToCheckout(blobPath, resolvedCheckoutInfo.absolutePath, fileSpec.Path); err != nil {
			return "", fmt.Errorf("symlinking %q to checkout: %w", fileSpec.Path, err)
		}
	}

	checkoutOK = true // Prevent the checkout directory from being erased again.
	logger.Info().Msg("shaman: checkout created")
	return resolvedCheckoutInfo.RelativePath, nil
}

func isValidCheckoutPath(checkoutPath string) bool {
	if !validCheckoutRegexp.MatchString(checkoutPath) {
		return false
	}
	if strings.Contains(checkoutPath, "../") || strings.Contains(checkoutPath, "/..") {
		return false
	}
	return true
}

// sanitizePaths ensures paths in 'checkout' use forward slashes only.
// Returns the sanitized checkout.
func sanitizePaths(checkout api.ShamanCheckout) api.ShamanCheckout {
	sane := api.ShamanCheckout{
		CheckoutPath: crosspath.ToSlash(checkout.CheckoutPath),
		Files:        slices.Clone(checkout.Files),
	}
	for i := range checkout.Files {
		sane.Files[i].Path = crosspath.ToSlash(sane.Files[i].Path)
	}
	return sane
}
