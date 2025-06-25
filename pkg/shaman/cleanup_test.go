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

package shaman

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projects.blender.org/studio/flamenco/pkg/shaman/config"
	"projects.blender.org/studio/flamenco/pkg/shaman/filestore"
	"projects.blender.org/studio/flamenco/pkg/shaman/jwtauth"
	"projects.blender.org/studio/flamenco/pkg/shaman/testsupport"
)

func createTestShaman() (*Server, func()) {
	output := zerolog.ConsoleWriter{Out: colorable.NewColorableStdout(), TimeFormat: time.RFC3339}
	log.Logger = log.Output(output)

	conf, confCleanup := config.CreateTestConfig()
	shaman := NewServer(conf, jwtauth.AlwaysDeny{})
	return shaman, confCleanup
}

func makeOld(shaman *Server, expectOld mtimeMap, relPath string) {
	if shaman.config.GarbageCollect.MaxAge < 2 {
		panic(fmt.Sprintf(
			"shaman.config.GarbageCollect.MaxAge is unusably low: %v",
			shaman.config.GarbageCollect.MaxAge))
	}
	age := -2 * shaman.config.GarbageCollect.MaxAge
	oldTime := time.Now().Add(age)
	absPath := filepath.Join(shaman.config.FileStorePath(), relPath)

	err := os.Chtimes(absPath, oldTime, oldTime)
	if err != nil {
		panic(err)
	}

	// Do a stat on the file to get the actual on-disk mtime (could be rounded/truncated).
	stat, err := os.Stat(absPath)
	if err != nil {
		panic(err)
	}
	osModTime := stat.ModTime()
	expectOld[absPath] = osModTime

	log.Debug().
		Str("relPath", relPath).
		Stringer("age", age).
		Stringer("stamp", oldTime).
		Stringer("actual", osModTime).
		Msg("makeOld")

	// Sanity check that the timestamp on disk is somewhat similar to what we expected.
	timediff := osModTime.Sub(oldTime).Abs()
	if timediff.Seconds() > 1 {
		panic(fmt.Sprintf("unable to set timestamp of %s:\n      set=%q but\n   actual=%q, difference is %s",
			absPath, oldTime, osModTime, timediff))
	}
}

func TestGCCanary(t *testing.T) {
	server, cleanup := createTestShaman()
	defer cleanup()

	assert.True(t, server.config.GarbageCollect.MaxAge > 10*time.Minute,
		"config.GarbageCollect.MaxAge must be big enough for this test to be reliable, is %v",
		server.config.GarbageCollect.MaxAge)
}

func TestGCFindOldFiles(t *testing.T) {
	server, cleanup := createTestShaman()
	defer cleanup()

	filestore.LinkTestFileStore(server.config.FileStorePath())

	// Since all the links have just been created, nothing should be considered old.
	ageThreshold := server.gcAgeThreshold()
	old, err := server.gcFindOldFiles(ageThreshold, log.With().Str("test", "test").Logger())
	require.NoError(t, err)
	assert.EqualValues(t, mtimeMap{}, old)

	// Make some files old, they should show up in a scan.
	expectOld := mtimeMap{}
	makeOld(server, expectOld, "stored/59/0c148428d5c35fab3ebad2f3365bb469ab9c531b60831f3e826c472027a0b9/3367.blob")
	makeOld(server, expectOld, "stored/80/b749c27b2fef7255e7e7b3c2029b03b31299c75ff1f1c72732081c70a713a3/7488.blob")
	makeOld(server, expectOld, "stored/dc/89f15de821ad1df3e78f8ef455e653a2d1862f2eb3f5ee78aa4ca68eb6fb35/781.blob")

	old, err = server.gcFindOldFiles(ageThreshold, log.With().Str("package", "shaman/test").Logger())
	require.NoError(t, err)
	assert.EqualValues(t, expectOld, old)
}

// Test of the lower-level functions of the garbage collector.
func TestGCComponents(t *testing.T) {
	testsupport.SkipTestIfUnableToSymlink(t)

	server, cleanup := createTestShaman()
	defer cleanup()

	filestore.LinkTestFileStore(server.config.FileStorePath())

	copymap := func(somemap mtimeMap) mtimeMap {
		theCopy := mtimeMap{}
		for key, value := range somemap {
			theCopy[key] = value
		}
		return theCopy
	}

	// Make some files old.
	expectOld := mtimeMap{}
	makeOld(server, expectOld, "stored/30/928ffced04c7008f3324fded86d133effea50828f5ad896196f2a2e190ac7e/6001.blob")
	makeOld(server, expectOld, "stored/59/0c148428d5c35fab3ebad2f3365bb469ab9c531b60831f3e826c472027a0b9/3367.blob")
	makeOld(server, expectOld, "stored/80/b749c27b2fef7255e7e7b3c2029b03b31299c75ff1f1c72732081c70a713a3/7488.blob")

	// utility mapping to be able to find absolute paths more easily
	absPaths := map[string]string{}
	for absPath := range expectOld {
		absPaths[filepath.Base(absPath)] = absPath
	}

	// No symlinks created yet, so this should report all the files in oldFiles.
	oldFiles := copymap(expectOld)
	err := server.gcFilterLinkedFiles(server.config.CheckoutPath(), oldFiles, log.With().Str("package", "shaman/test").Logger(), nil)
	require.NoError(t, err)
	assert.EqualValues(t, expectOld, oldFiles)

	// Create some symlinks
	checkoutInfo, err := server.checkoutMan.PrepareCheckout("checkoutID")
	require.NoError(t, err)
	err = server.checkoutMan.SymlinkToCheckout(absPaths["3367.blob"], server.config.CheckoutPath(),
		filepath.Join(checkoutInfo.RelativePath, "use-of-3367.blob"))
	require.NoError(t, err)

	// There should be two old file reported.
	expectRemovable := mtimeMap{
		absPaths["6001.blob"]: expectOld[absPaths["6001.blob"]],
		absPaths["7488.blob"]: expectOld[absPaths["7488.blob"]],
	}
	oldFiles = copymap(expectOld)
	stats := GCStats{}
	err = server.gcFilterLinkedFiles(server.config.CheckoutPath(), oldFiles, log.With().Str("package", "shaman/test").Logger(), &stats)
	assert.Equal(t, 1, stats.numSymlinksChecked)
	require.NoError(t, err)
	assert.Equal(t, len(expectRemovable), len(oldFiles))

	// Touching a file before requesting deletion should prevent its deletetion.
	now := time.Now()
	err = os.Chtimes(absPaths["6001.blob"], now, now)
	require.NoError(t, err)

	// Running the garbage collector should only remove that one unused and untouched file.
	assert.FileExists(t, absPaths["6001.blob"], "file should exist before GC")
	assert.FileExists(t, absPaths["7488.blob"], "file should exist before GC")
	server.gcDeleteOldFiles(true, oldFiles, log.With().Str("package", "shaman/test").Logger())
	assert.FileExists(t, absPaths["6001.blob"], "file should exist after dry-run GC")
	assert.FileExists(t, absPaths["7488.blob"], "file should exist after dry-run GC")

	server.gcDeleteOldFiles(false, oldFiles, log.With().Str("package", "shaman/test").Logger())

	assert.FileExists(t, absPaths["3367.blob"], "file should exist after GC")
	assert.FileExists(t, absPaths["6001.blob"], "file should exist after GC")
	_, err = os.Stat(absPaths["7488.blob"])
	assert.ErrorIs(t, err, fs.ErrNotExist, "file %s should NOT exist after GC", absPaths["7488.blob"])
}

// Test of the high-level GCStorage() function.
func TestGarbageCollect(t *testing.T) {
	testsupport.SkipTestIfUnableToSymlink(t)

	server, cleanup := createTestShaman()
	defer cleanup()

	filestore.LinkTestFileStore(server.config.FileStorePath())

	// Make some files old.
	expectOld := mtimeMap{}
	makeOld(server, expectOld, "stored/30/928ffced04c7008f3324fded86d133effea50828f5ad896196f2a2e190ac7e/6001.blob")
	makeOld(server, expectOld, "stored/59/0c148428d5c35fab3ebad2f3365bb469ab9c531b60831f3e826c472027a0b9/3367.blob")
	makeOld(server, expectOld, "stored/80/b749c27b2fef7255e7e7b3c2029b03b31299c75ff1f1c72732081c70a713a3/7488.blob")

	// utility mapping to be able to find absolute paths more easily
	absPaths := map[string]string{}
	for absPath := range expectOld {
		absPaths[filepath.Base(absPath)] = absPath
	}

	// Create some symlinks
	checkoutInfo, err := server.checkoutMan.PrepareCheckout("checkoutID")
	require.NoError(t, err)
	err = server.checkoutMan.SymlinkToCheckout(absPaths["3367.blob"], server.config.CheckoutPath(),
		filepath.Join(checkoutInfo.RelativePath, "use-of-3367.blob"))
	require.NoError(t, err)

	// Running the garbage collector should only remove those two unused files.
	assert.FileExists(t, absPaths["6001.blob"], "file should exist before GC")
	assert.FileExists(t, absPaths["7488.blob"], "file should exist before GC")
	server.GCStorage(true)
	assert.FileExists(t, absPaths["6001.blob"], "file should exist after dry-run GC")
	assert.FileExists(t, absPaths["7488.blob"], "file should exist after dry-run GC")
	server.GCStorage(false)
	_, err = os.Stat(absPaths["6001.blob"])
	assert.ErrorIs(t, err, fs.ErrNotExist, "file %s should NOT exist after GC", absPaths["6001.blob"])
	_, err = os.Stat(absPaths["7488.blob"])
	assert.ErrorIs(t, err, fs.ErrNotExist, "file %s should NOT exist after GC", absPaths["7488.blob"])

	// Used files should still exist.
	assert.FileExists(t, absPaths["3367.blob"])
}
