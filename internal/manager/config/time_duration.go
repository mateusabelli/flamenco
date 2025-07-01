package config

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"encoding/json"
	"errors"
	"time"

	yaml "gopkg.in/yaml.v2"
)

// Duration is a time.Duration with custom JSON/YAML marshallers.
type Duration time.Duration

var _ json.Unmarshaler = (*Duration)(nil)
var _ json.Marshaler = Duration(0)
var _ yaml.Unmarshaler = (*Duration)(nil)
var _ yaml.Marshaler = Duration(0)

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	return d.unmarshal(v)
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	stringValue := ""
	if err := unmarshal(&stringValue); err != nil {
		return err
	}
	return d.unmarshal(stringValue)
}

func (d *Duration) unmarshal(v any) error {
	switch value := v.(type) {
	case string:
		timeDuration, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(timeDuration)
		return nil
	default:
		return errors.New("invalid duration")
	}
}
