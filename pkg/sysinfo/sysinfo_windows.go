package sysinfo

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// canSymlink tries to determine whether the running system can use symbolic
// links, based on information from the Windows registry.
func canSymlink() (bool, error) {
	if isDeveloperModeActive() {
		return true, nil
	}
	return hasSystemPrivilege("SeCreateSymbolicLinkPrivilege")
}

func description() (string, error) {
	productName, err := registryReadString(
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`,
		"ProductName",
	)
	if err != nil {
		return "", err
	}

	editionID, err := windowsEditionID()
	if err != nil {
		return "", err
	}

	description := fmt.Sprintf("%s (%s)", productName, editionID)
	return description, nil
}

func windowsEditionID() (string, error) {
	// Values seen so far:
	// - "Professional"
	// - "Core"
	return registryReadString(
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`,
		"EditionID",
	)
}

func registryReadString(keyPath, valueName string) (value string, err error) {
	regkey, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		keyPath,
		registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("opening registry key %q: %w", keyPath, err)
	}
	defer func() {
		if closeErr := regkey.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing registry key %q: %w", keyPath, closeErr)
		}
	}()

	value, _, err = regkey.GetStringValue(valueName)
	if err != nil {
		return "", fmt.Errorf("reading registry key %q: %w", valueName, err)
	}

	return value, nil
}

// isDeveloperModeActive checks whether or not the developer mode is active on Windows 10.
// Returns false for prior Windows versions.
// see https://docs.microsoft.com/en-us/windows/uwp/get-started/enable-your-device-for-development
// Copied from https://github.com/golang/go/pull/24307/files
func isDeveloperModeActive() bool {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock`,
		registry.READ)
	if err != nil {
		return false
	}

	val, _, err := key.GetIntegerValue("AllowDevelopmentWithoutDevLicense")
	if err != nil {
		return false
	}

	return val != 0
}

// hasSystemPrivilege checks whether the user has the
// SeCreateSymbolicLinkPrivilege, which is necessary to create symbolic links.
func hasSystemPrivilege(privilegeName string) (bool, error) {
	// This doesn't fail, and just returns -1. The Microsoft docs still recommend
	// calling this function, though, instead of just hard-coding the -1 value.
	hProcess := windows.CurrentProcess()

	// Open a process token, necessary for the subsequent calls.
	var processToken windows.Token
	err := windows.OpenProcessToken(hProcess, windows.TOKEN_READ, &processToken)
	if err != nil {
		return false, fmt.Errorf("calling OpenProcessToken: %w", err)
	}
	defer func() {
		_ = processToken.Close()
	}()

	privilegeNameU16, err := syscall.UTF16PtrFromString(privilegeName)
	if err != nil {
		return false, fmt.Errorf("invalid privilege name %q: %w", privilegeName, err)
	}

	// Look up the LUID for the privilege.
	var privilegeLUID windows.LUID
	err = windows.LookupPrivilegeValue(nil, privilegeNameU16, &privilegeLUID)
	if err != nil {
		return false, fmt.Errorf("calling LookupPrivilegeValue: %w", err)
	}

	// Get the size of the buffer needed for the actual data.
	var TokenPrivileges uint32 = 3
	var bufferSize uint32
	err = windows.GetTokenInformation(processToken, TokenPrivileges, nil, 0, &bufferSize)
	if errno, ok := err.(syscall.Errno); !ok || errno != 122 {
		return false, fmt.Errorf("unexpected error from first GetTokenInformation call: %w", err)
	}

	// Get the list of user's privileges.
	buffer := make([]byte, bufferSize)
	err = windows.GetTokenInformation(processToken, TokenPrivileges, &buffer[0], bufferSize, &bufferSize)
	if err != nil {
		return false, fmt.Errorf("unexpected error from second GetTokenInformation call: %w", err)
	}

	// Decode the privileges.
	privCount := int(binary.LittleEndian.Uint32(buffer))
	offset := int(unsafe.Sizeof(uint32(0)))
	structSize := int(unsafe.Sizeof(windows.LUIDAndAttributes{}))

	for i := 0; i < privCount; i++ {
		structPtr := &buffer[offset+structSize*i]
		luidAndAttr := *(*windows.LUIDAndAttributes)(unsafe.Pointer(structPtr))
		if luidAndAttr.Luid != privilegeLUID {
			continue
		}

		log.Debug().Str("privilege", privilegeName).Msg("found privilege")
		return true, nil
	}

	return false, nil
}
