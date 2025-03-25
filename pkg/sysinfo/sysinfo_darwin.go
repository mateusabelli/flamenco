package sysinfo

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

type PlistData struct {
	XMLName xml.Name `xml:"plist"`
	Dict    Dict     `xml:"dict"`
}

type Dict struct {
	Keys    []string `xml:"key"`
	Strings []string `xml:"string"`
}

// canSymlink always returns true, as symlinking on non-Windows platforms is not hard.
func canSymlink() (bool, error) {
	return true, nil
}

func description() (string, error) {
	plistFile := "/System/Library/CoreServices/SystemVersion.plist"
	info, err := getSystemInfo(plistFile)
	if err != nil {
		log.Warn().Err(err).Msg("Could not retrieve system information")
		return "macOS", nil
	}
	return info, nil
}

func getSystemInfo(plistFile string) (string, error) {
	data, err := os.ReadFile(plistFile)
	if err != nil {
		return "", fmt.Errorf("could not read system info file %s: %w", plistFile, err)
	}

	var plist PlistData
	if err := xml.Unmarshal(data, &plist); err != nil {
		return "", fmt.Errorf("failed to read system info from %s: %w", plistFile, err)
	}

	productName := "macOS"
	var productVersion, buildVersion string

	for i, key := range plist.Dict.Keys {
		if i >= len(plist.Dict.Strings) {
			break
		}
		switch key {
		case "ProductName":
			productName = plist.Dict.Strings[i]
		case "ProductVersion":
			productVersion = plist.Dict.Strings[i]
		case "ProductBuildVersion":
			buildVersion = plist.Dict.Strings[i]
		}
	}

	parts := []string{productName}
	if productVersion != "" {
		parts = append(parts, productVersion)
	}
	if buildVersion != "" {
		parts = append(parts, fmt.Sprintf("(Build %s)", buildVersion))
	}

	return strings.Join(parts, " "), nil
}
