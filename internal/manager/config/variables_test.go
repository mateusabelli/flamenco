package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplaceTwowayVariablesMixedSlashes(t *testing.T) {
	c := DefaultConfig(func(c *Conf) {
		c.Variables["shared"] = Variable{
			IsTwoWay: true,
			Values: []VariableValue{
				{Value: "/shared/flamenco", Platform: VariablePlatformLinux},
				{Value: `Y:\shared\flamenco`, Platform: VariablePlatformWindows},
			},
		}
	})

	replacerWin := c.NewVariableToValueConverter(VariableAudienceWorkers, VariablePlatformWindows)
	replacerLnx := c.NewVariableToValueConverter(VariableAudienceWorkers, VariablePlatformLinux)

	// This is the real reason for this test: forward slashes in the path should
	// still be matched to the backslashes in the variable value.
	assert.Equal(t, `{shared}\shot\file.blend`, replacerWin.Replace(`Y:\shared\flamenco\shot\file.blend`))
	assert.Equal(t, `{shared}/shot/file.blend`, replacerWin.Replace(`Y:/shared/flamenco/shot/file.blend`))

	assert.Equal(t, `{shared}\shot\file.blend`, replacerLnx.Replace(`/shared\flamenco\shot\file.blend`))
	assert.Equal(t, `{shared}/shot/file.blend`, replacerLnx.Replace(`/shared/flamenco/shot/file.blend`))
}

func TestExpandTwowayVariablesMixedSlashes(t *testing.T) {
	c := DefaultConfig(func(c *Conf) {
		c.Variables["shared"] = Variable{
			IsTwoWay: true,
			Values: []VariableValue{
				{Value: "/shared/flamenco", Platform: VariablePlatformLinux},
				{Value: `Y:\shared\flamenco`, Platform: VariablePlatformWindows},
			},
		}
	})

	expanderWin := c.NewVariableExpander(VariableAudienceWorkers, VariablePlatformWindows)
	expanderLnx := c.NewVariableExpander(VariableAudienceWorkers, VariablePlatformLinux)

	// Slashes should always be normalised for the target platform, on the entire path, not just the replaced part.
	assert.Equal(t, `Y:\shared\flamenco\shot\file.blend`, expanderWin.Expand(`{shared}\shot\file.blend`))
	assert.Equal(t, `Y:\shared\flamenco\shot\file.blend`, expanderWin.Expand(`{shared}/shot/file.blend`))

	assert.Equal(t, `/shared/flamenco/shot/file.blend`, expanderLnx.Expand(`{shared}\shot\file.blend`))
	assert.Equal(t, `/shared/flamenco/shot/file.blend`, expanderLnx.Expand(`{shared}/shot/file.blend`))
}
