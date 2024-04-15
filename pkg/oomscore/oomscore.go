// package oomscore provides some functions to adjust the Linux
// out-of-memory (OOM) score, i.e. the number that determines how likely it is
// that a process is killed in an out-of-memory situation.
//
// It is available only on Linux. On other platforms ErrNotImplemented will be returned.
package oomscore

import (
	"errors"

	"github.com/rs/zerolog/log"
)

var ErrNotImplemented = errors.New("OOM score functionality not implemented on this platform")

// Available returns whether the functionality in this package is available for
// the current platform.
func Available() bool {
	return available
}

// GetOOMScore returns the current process' OOM score.
func GetOOMScore() (int, error) {
	return getOOMScore()
}

// GetOOMScoreAdj returns the current process' OOM score adjustment.
func GetOOMScoreAdj() (int, error) {
	return getOOMScoreAdj()
}

// SetOOMScoreAdj sets the current process' OOM score adjustment.
func SetOOMScoreAdj(score int) error {
	return setOOMScoreAdj(score)
}

type ScoreRestoreFunc func()

var emptyRestoreFunc ScoreRestoreFunc = func() {}

// Adjust temporarily sets the OOM score adjustment.
// The returned function MUST be called to restore the original value.
// Any problems changing the score are logged, but not otherwise returned.
func Adjust(score int) (restoreFunc ScoreRestoreFunc) {
	restoreFunc = emptyRestoreFunc

	if !Available() {
		return
	}

	origScore, err := getOOMScoreAdj()
	if err != nil {
		log.Error().
			AnErr("cause", err).
			Msg("could not get the current process' oom_score_adj value")
		return
	}

	log.Trace().
		Int("oom_score_adj", score).
		Msg("setting oom_score_adj")

	err = setOOMScoreAdj(score)
	if err != nil {
		log.Error().
			Int("oom_score_adj", score).
			AnErr("cause", err).
			Msg("could not set the current process' oom_score_adj value")
		return
	}

	return func() {
		log.Trace().
			Int("oom_score_adj", origScore).
			Msg("restoring oom_score_adj")

		err = setOOMScoreAdj(origScore)
		if err != nil {
			log.Error().
				Int("oom_score_adj", origScore).
				AnErr("cause", err).
				Msg("could not restore the current process' oom_score_adj value")
			return
		}
	}
}
