package sysinfo

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSystemInfo_ValidPlist(t *testing.T) {
	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
	<plist version="1.0">
		<dict>
			<key>ProductName</key>
			<string>macOS</string>
			<key>ProductVersion</key>
			<string>15.3.1</string>
			<key>ProductBuildVersion</key>
			<string>24D70</string>
		</dict>
	</plist>`

	tempFile, cleanup := createTempPlist(t, plistContent)
	defer cleanup()

	expected := "macOS 15.3.1 (Build 24D70)"
	result, err := getSystemInfo(tempFile)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestGetSystemInfo_NoProductName(t *testing.T) {
	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
	<plist version="1.0">
		<dict>
			<key>ProductVersion</key>
			<string>15.3.1</string>
			<key>ProductBuildVersion</key>
			<string>24D70</string>
		</dict>
	</plist>`

	tempFile, cleanup := createTempPlist(t, plistContent)
	defer cleanup()

	expected := "macOS 15.3.1 (Build 24D70)"
	result, err := getSystemInfo(tempFile)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestGetSystemInfo_OnlyProductName(t *testing.T) {
	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
	<plist version="1.0">
		<dict>
			<key>ProductName</key>
			<string>macOS Custom</string>
		</dict>
	</plist>`

	tempFile, cleanup := createTempPlist(t, plistContent)
	defer cleanup()

	expected := "macOS Custom"
	result, err := getSystemInfo(tempFile)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestGetSystemInfo_EmptyDict(t *testing.T) {
	plistContent := `<?xml version="1.0" encoding="UTF-8"?>
	<plist version="1.0">
		<dict></dict>
	</plist>`

	tempFile, cleanup := createTempPlist(t, plistContent)
	defer cleanup()

	expected := "macOS"
	result, err := getSystemInfo(tempFile)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestGetSystemInfo_InvalidXML(t *testing.T) {
	plistContent := `INVALID_XML_DATA`

	tempFile, cleanup := createTempPlist(t, plistContent)
	defer cleanup()

	_, err := getSystemInfo(tempFile)

	assert.Error(t, err)
}

func TestGetSystemInfo_FileNotFound(t *testing.T) {
	_, err := getSystemInfo("/path/to/nonexistent.plist")

	assert.Error(t, err)
}

func createTempPlist(t *testing.T, content string) (string, func()) {
	tempFile, err := os.CreateTemp("", "test_plist_*.plist")
	assert.NoError(t, err)

	_, err = tempFile.WriteString(content)
	assert.NoError(t, err)

	err = tempFile.Close()
	assert.NoError(t, err)

	cleanup := func() {
		os.Remove(tempFile.Name())
	}

	return tempFile.Name(), cleanup
}
