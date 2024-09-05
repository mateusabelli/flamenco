/* (c) 2019, Blender Foundation - Sybren A. Stüvel
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
 * CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package checkout

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projects.blender.org/studio/flamenco/pkg/api"
	"projects.blender.org/studio/flamenco/pkg/shaman/config"
	"projects.blender.org/studio/flamenco/pkg/shaman/filestore"
	"projects.blender.org/studio/flamenco/pkg/shaman/testsupport"
)

func createTestManager() (*Manager, func()) {
	output := zerolog.ConsoleWriter{Out: colorable.NewColorableStdout(), TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)

	conf, confCleanup := config.CreateTestConfig()
	fileStore := filestore.New(conf)
	manager := NewManager(conf, fileStore)
	return manager, confCleanup
}

func TestSymlinkToCheckout(t *testing.T) {
	testsupport.SkipTestIfUnableToSymlink(t)

	manager, cleanup := createTestManager()
	defer cleanup()

	// Fake an older file.
	blobPath := filepath.Join(manager.fileStore.StoragePath(), "opjehoofd.blob")
	err := os.WriteFile(blobPath, []byte("op je hoofd"), 0600)
	require.NoError(t, err)

	wayBackWhen := time.Now().Add(-time.Hour * 24 * 100)
	err = os.Chtimes(blobPath, wayBackWhen, wayBackWhen)
	require.NoError(t, err)

	symlinkRelativePath := "path/to/opjehoofd.txt"
	err = manager.SymlinkToCheckout(blobPath, manager.checkoutBasePath, symlinkRelativePath)
	require.NoError(t, err)

	err = manager.SymlinkToCheckout(blobPath, manager.checkoutBasePath, symlinkRelativePath)
	require.NoError(t, err, "symlinking a file twice should not be an issue")

	// Wait for the manager to be done updating mtimes.
	manager.wg.Wait()

	// The blob should have been touched to indicate it was referenced just now.
	stat, err := os.Stat(blobPath)
	require.NoError(t, err)
	assert.True(t,
		stat.ModTime().After(wayBackWhen),
		"File must be touched (%v must be later than %v)", stat.ModTime(), wayBackWhen)

	symlinkPath := filepath.Join(manager.checkoutBasePath, symlinkRelativePath)
	stat, err = os.Lstat(symlinkPath)
	require.NoError(t, err)
	assert.True(t, stat.Mode()&os.ModeType == os.ModeSymlink,
		"%v should be a symlink", symlinkPath)

	contents, err := os.ReadFile(symlinkPath)
	require.NoError(t, err)
	assert.Equal(t, string(contents), "op je hoofd")
}

func TestPrepareCheckout(t *testing.T) {
	manager, cleanup := createTestManager()
	defer cleanup()

	requestedCheckoutPath := "some-path/that is/unique/at first"

	// On first call, this path should be unique.
	resolved, err := manager.PrepareCheckout(requestedCheckoutPath)
	require.NoError(t, err)
	assert.Equal(t, requestedCheckoutPath, resolved.RelativePath)

	// At the second call, it already exists and thus should be altered with a random suffix.
	resolved, err = manager.PrepareCheckout(requestedCheckoutPath)
	require.NoError(t, err)
	assert.NotEqual(t, requestedCheckoutPath, resolved.RelativePath)
	assert.True(t, strings.HasPrefix(resolved.RelativePath, requestedCheckoutPath+"-"))
}

func TestEraseCheckout(t *testing.T) {
	testsupport.SkipTestIfUnableToSymlink(t)

	manager, cleanup := createTestManager()
	defer cleanup()
	ctx := context.Background()

	filestore.LinkTestFileStore(manager.fileStore.BasePath())

	// Create a few checkouts to test with.
	checkout1 := api.ShamanCheckout{
		CheckoutPath: "á hausinn á þér",
		Files: []api.ShamanFileSpec{
			{Sha: "590c148428d5c35fab3ebad2f3365bb469ab9c531b60831f3e826c472027a0b9", Size: 3367, Path: "subdir/replacer.py"},
			{Sha: "80b749c27b2fef7255e7e7b3c2029b03b31299c75ff1f1c72732081c70a713a3", Size: 7488, Path: "feed.py"},
			{Sha: "914853599dd2c351ab7b82b219aae6e527e51518a667f0ff32244b0c94c75688", Size: 486, Path: "httpstuff.py"},
			{Sha: "d6fc7289b5196cc96748ea72f882a22c39b8833b457fe854ef4c03a01f5db0d3", Size: 7217, Path: "много ликова.py"},
		},
	}
	checkoutID1, err := manager.Checkout(ctx, checkout1)
	require.NoError(t, err)

	checkout2 := checkout1
	checkout2.CheckoutPath = "één ander pad"
	checkoutID2, err := manager.Checkout(ctx, checkout2)
	require.NoError(t, err)

	// Check that removing one works, without deleting the other.
	require.NoError(t, manager.EraseCheckout(checkoutID1))

	checkoutPath1, err := manager.pathForCheckout(checkoutID1)
	require.NoError(t, err)
	checkoutPath2, err := manager.pathForCheckout(checkoutID2)
	require.NoError(t, err)

	assert.NoDirExists(t, checkoutPath1.absolutePath, "actual checkout path should have been erased")
	assert.DirExists(t, checkoutPath2.absolutePath, "the other checkout path should have been kept")
	assert.DirExists(t, manager.fileStore.StoragePath(), "Shaman storage path should be kept")

	// Check that non-checkout paths should be refused.
	require.Error(t, manager.EraseCheckout(manager.fileStore.BasePath()))
}

func TestEraseCheckoutNonExisting(t *testing.T) {
	manager, cleanup := createTestManager()
	defer cleanup()

	filestore.LinkTestFileStore(manager.fileStore.BasePath())

	// Erasing a non-existing checkout should return a specific error.
	require.Error(t, manager.EraseCheckout("não existe"))
}
