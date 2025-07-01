package config

// SPDX-License-Identifier: GPL-3.0-or-later

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestDuration_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		want    Duration
		input   []byte
		wantErr bool
	}{
		{"60s", Duration(time.Second * 60), []byte(`"60s"`), false},
		{"1m", Duration(time.Second * 60), []byte(`"1m"`), false},
		{"int", Duration(0), []byte("1"), true},
		{"float", Duration(0), []byte("1.0"), true},
		{"empty", Duration(0), []byte{}, true},
		{"undefined", Duration(0), []byte("undefined"), true},
		{"null", Duration(0), []byte("null"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Duration
			err := got.UnmarshalJSON(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("Duration.UnmarshalJSON(%v) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Duration.UnmarshalJSON(%v) got = %v, want = %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDuration_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		want    []byte
		input   Duration
		wantErr bool
	}{
		{"zero", []byte(`"0s"`), Duration(0), false},
		{"1ns", []byte(`"1ns"`), Duration(1), false},
		{"1m", []byte(`"1m0s"`), Duration(time.Second * 60), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.MarshalJSON()

			if (err != nil) != tt.wantErr {
				t.Errorf("Duration.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Duration.MarshalJSON() got = %v, want = %v", string(got), string(tt.want))
			}
		})
	}
}

func TestDuration_JSONDocument(t *testing.T) {
	type TestStruct struct {
		TestValue Duration `json:"test_value"`
	}

	testValue := TestStruct{Duration(time.Hour * 3)}
	jsonBytes, err := json.Marshal(testValue)
	assert.NoError(t, err)

	assert.Equal(t, `{"test_value":"3h0m0s"}`, string(jsonBytes))

	roundtripValue := TestStruct{}
	err = json.Unmarshal(jsonBytes, &roundtripValue)
	assert.NoError(t, err)
	assert.Equal(t, testValue, roundtripValue)
}

func TestDuration_YAMLDocument(t *testing.T) {
	type TestStruct struct {
		TestValue Duration `yaml:"test_value"`
	}

	testValue := TestStruct{Duration(time.Hour * 3)}
	yamlBytes, err := yaml.Marshal(testValue)
	assert.NoError(t, err)

	assert.Equal(t, "test_value: 3h0m0s\n", string(yamlBytes))

	roundtripValue := TestStruct{}
	err = yaml.Unmarshal(yamlBytes, &roundtripValue)
	assert.NoError(t, err)
	assert.Equal(t, testValue, roundtripValue)
}
