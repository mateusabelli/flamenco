//go:build !linux

package oomscore

const (
	available = false
)

func getOOMScore() (int, error) {
	return 0, ErrNotImplemented
}

func getOOMScoreAdj() (int, error) {
	return 0, ErrNotImplemented
}

func setOOMScoreAdj(int) error {
	return ErrNotImplemented
}
