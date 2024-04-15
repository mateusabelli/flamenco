//go:build linux

package oomscore

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	available = true
)

// getOOMScore returns the current process' OOM score.
func getOOMScore() (int, error) {
	return readInt("oom_score")
}

// getOOMScoreAdj returns the current process' OOM score adjustment.
func getOOMScoreAdj() (int, error) {
	return readInt("oom_score_adj")
}

// setOOMScoreAdj sets the current process' OOM score adjustment.
func setOOMScoreAdj(newScore int) error {
	return writeInt(newScore, "oom_score_adj")
}

// readInt reads an integer from /proc/{pid}/{filename}
func readInt(filename string) (int, error) {
	fullPath := procPidPath(filename)

	file, err := os.Open(fullPath)
	if err != nil {
		return 0, fmt.Errorf("opening %s: %w", fullPath, err)
	}

	var valueInFile int
	n, err := fmt.Fscan(file, &valueInFile)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", fullPath, err)
	}
	if n < 1 {
		return 0, fmt.Errorf("reading %s: did not find a number", fullPath)
	}

	return valueInFile, nil
}

// writeInt writes an integer to /proc/{pid}/{filename}
func writeInt(value int, filename string) error {
	fullPath := procPidPath(filename)
	contents := fmt.Sprint(value)
	err := os.WriteFile(fullPath, []byte(contents), os.ModePerm)
	if err != nil {
		return fmt.Errorf("writing %s: %w", fullPath, err)
	}
	return nil
}

// procPidPath returns "/proc/{pid}/{filename}".
func procPidPath(filename string) string {
	pid := os.Getpid()
	return filepath.Join("/proc", fmt.Sprint(pid), filename)
}
