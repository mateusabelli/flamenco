package config

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"projects.blender.org/studio/flamenco/pkg/crosspath"
)

type ValueToVariableReplacer struct {
	twoWayVars map[string]string // Mapping from variable name to value.
}

// VariableExpander expands variables and applies two-way variable replacement to the values.
type VariableExpander struct {
	oneWayVars        map[string]string // Mapping from variable name to value.
	managerTwoWayVars map[string]string // Mapping from variable name to value for the Manager platform.
	targetTwoWayVars  map[string]string // Mapping from variable name to value for the target platform.
	targetPlatform    VariablePlatform
}

// NewVariableToValueConverter returns a ValueToVariableReplacer for the given audience & platform.
func (c *Conf) NewVariableToValueConverter(audience VariableAudience, platform VariablePlatform) *ValueToVariableReplacer {
	// Get the variables for the given audience & platform.
	twoWayVars := c.GetTwoWayVariables(audience, platform)

	if len(twoWayVars) == 0 {
		log.Debug().
			Str("audience", string(audience)).
			Str("platform", string(platform)).
			Msg("no two-way variables defined for this platform given this audience")
	}

	return &ValueToVariableReplacer{
		twoWayVars: twoWayVars,
	}
}

// NewVariableExpander returns a new VariableExpander for the given audience & platform.
func (c *Conf) NewVariableExpander(audience VariableAudience, platform VariablePlatform) *VariableExpander {
	return &VariableExpander{
		oneWayVars:        varsForPlatform,
		managerTwoWayVars: c.GetTwoWayVariables(audience, c.currentGOOS),
		targetTwoWayVars:  c.GetTwoWayVariables(audience, platform),
		targetPlatform:    platform,
	}
}

// ValueToVariableReplacer replaces any variable values it recognises in
// valueToConvert to the actual variable. For example, `/path/to/file.blend` can
// be changed to `{my_storage}/file.blend`.
func (vvc *ValueToVariableReplacer) Replace(valueToConvert string) string {
	result := valueToConvert

	for varName, varValue := range vvc.twoWayVars {
		if !isValueMatch(result, varValue) {
			continue
		}
		result = vvc.join(varName, result[len(varValue):])
	}

	log.Debug().
		Str("from", valueToConvert).
		Str("to", result).
		Msg("first step of two-way variable replacement")

	return result
}

func (vvc *ValueToVariableReplacer) join(varName, value string) string {
	return fmt.Sprintf("{%s}%s", varName, value)
}

// isValueMatch returns whether `valueToMatch` starts with `variableValue`.
// When `variableValue` is a Windows path (with backslash separators), it is
// also tested with forward slashes against `valueToMatch`.
func isValueMatch(valueToMatch, variableValue string) bool {
	if strings.HasPrefix(valueToMatch, variableValue) {
		return true
	}

	// If the variable value has a backslash, assume it is a Windows path.
	// Convert it to slash notation just to see if that would provide a
	// match.
	if strings.ContainsRune(variableValue, '\\') {
		slashedValue := crosspath.ToSlash(variableValue)
		return strings.HasPrefix(valueToMatch, slashedValue)
	}

	return false
}

// Replace converts "{variable name}" to the value that belongs to the audience and platform.
func (ve *VariableExpander) Expand(valueToExpand string) string {
	expanded := valueToExpand

	// Expand variables from {varname} to their value for the target platform.
	for varname, varvalue := range ve.oneWayVars {
		placeholder := fmt.Sprintf("{%s}", varname)
		expanded = strings.Replace(expanded, placeholder, varvalue, -1)
	}

	// Go through the two-way variables, to make sure that the result of
	// expanding variables gets the two-way variables applied as well. This is
	// necessary to make implicitly-defined variable, which are only defined for
	// the Manager's platform, usable for the target platform.
	//
	// Practically, this replaces "value for the Manager platform" with "value
	// for the target platform".
	isPathValue := false
	for varname, managerValue := range ve.managerTwoWayVars {
		targetValue, ok := ve.targetTwoWayVars[varname]
		if !ok {
			continue
		}
		if !isValueMatch(expanded, managerValue) {
			continue
		}
		expanded = ve.join(targetValue, expanded[len(managerValue):], ve.targetPlatform)

		// Since two-way variables are meant for path replacement, we know this
		// should be a path.
		isPathValue = true
	}

	if isPathValue {
		expanded = crosspath.ToPlatform(expanded, string(ve.targetPlatform))
	}

	return expanded
}

func (ve *VariableExpander) join(valueFromVariable, suffix string, platform VariablePlatform) string {
	result := valueFromVariable + suffix

	if platform == VariablePlatformWindows {
		// 'result' may now be of the form `F:some\path\to\file`, where `F:` comes
		// from `valueFromVariable` and the rest is the suffix. This is not an
		// absolute path, and needs a separator between the drive letter and the
		// rest of the path.
		return crosspath.EnsureDriveAbsolute(result)
	}

	return result
}
