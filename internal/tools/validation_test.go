/*-------------------------------------------------------------------------
 *
 * pgEdge Natural Language Agent
 *
 * Portions copyright (c) 2025, pgEdge, Inc.
 * This software is released under The PostgreSQL License
 *
 *-------------------------------------------------------------------------
 */

package tools

import (
	"testing"
)

func TestValidateStringParam(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		paramName string
		wantValue string
		wantError bool
	}{
		{
			name:      "valid string parameter",
			args:      map[string]interface{}{"test": "value"},
			paramName: "test",
			wantValue: "value",
			wantError: false,
		},
		{
			name:      "missing parameter",
			args:      map[string]interface{}{},
			paramName: "test",
			wantValue: "",
			wantError: true,
		},
		{
			name:      "empty string",
			args:      map[string]interface{}{"test": ""},
			paramName: "test",
			wantValue: "",
			wantError: true,
		},
		{
			name:      "wrong type (number)",
			args:      map[string]interface{}{"test": 123},
			paramName: "test",
			wantValue: "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotError := ValidateStringParam(tt.args, tt.paramName)

			if gotValue != tt.wantValue {
				t.Errorf("ValidateStringParam() value = %v, want %v", gotValue, tt.wantValue)
			}

			if (gotError != nil) != tt.wantError {
				t.Errorf("ValidateStringParam() error = %v, wantError %v", gotError != nil, tt.wantError)
			}

			if gotError != nil && !gotError.IsError {
				t.Error("ValidateStringParam() returned response should have IsError = true")
			}
		})
	}
}

func TestValidateOptionalStringParam(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]interface{}
		paramName    string
		defaultValue string
		want         string
	}{
		{
			name:         "present parameter",
			args:         map[string]interface{}{"test": "value"},
			paramName:    "test",
			defaultValue: "default",
			want:         "value",
		},
		{
			name:         "missing parameter",
			args:         map[string]interface{}{},
			paramName:    "test",
			defaultValue: "default",
			want:         "default",
		},
		{
			name:         "wrong type",
			args:         map[string]interface{}{"test": 123},
			paramName:    "test",
			defaultValue: "default",
			want:         "default",
		},
		{
			name:         "empty string returns empty",
			args:         map[string]interface{}{"test": ""},
			paramName:    "test",
			defaultValue: "default",
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateOptionalStringParam(tt.args, tt.paramName, tt.defaultValue)
			if got != tt.want {
				t.Errorf("ValidateOptionalStringParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateNumberParam(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		paramName string
		wantValue float64
		wantError bool
	}{
		{
			name:      "valid number",
			args:      map[string]interface{}{"test": 42.5},
			paramName: "test",
			wantValue: 42.5,
			wantError: false,
		},
		{
			name:      "missing parameter",
			args:      map[string]interface{}{},
			paramName: "test",
			wantValue: 0,
			wantError: true,
		},
		{
			name:      "wrong type (string)",
			args:      map[string]interface{}{"test": "123"},
			paramName: "test",
			wantValue: 0,
			wantError: true,
		},
		{
			name:      "zero is valid",
			args:      map[string]interface{}{"test": 0.0},
			paramName: "test",
			wantValue: 0,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotError := ValidateNumberParam(tt.args, tt.paramName)

			if gotValue != tt.wantValue {
				t.Errorf("ValidateNumberParam() value = %v, want %v", gotValue, tt.wantValue)
			}

			if (gotError != nil) != tt.wantError {
				t.Errorf("ValidateNumberParam() error = %v, wantError %v", gotError != nil, tt.wantError)
			}
		})
	}
}

func TestValidateOptionalNumberParam(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]interface{}
		paramName    string
		defaultValue float64
		want         float64
	}{
		{
			name:         "present parameter",
			args:         map[string]interface{}{"test": 42.5},
			paramName:    "test",
			defaultValue: 10.0,
			want:         42.5,
		},
		{
			name:         "missing parameter",
			args:         map[string]interface{}{},
			paramName:    "test",
			defaultValue: 10.0,
			want:         10.0,
		},
		{
			name:         "wrong type",
			args:         map[string]interface{}{"test": "123"},
			paramName:    "test",
			defaultValue: 10.0,
			want:         10.0,
		},
		{
			name:         "zero returns zero",
			args:         map[string]interface{}{"test": 0.0},
			paramName:    "test",
			defaultValue: 10.0,
			want:         0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateOptionalNumberParam(tt.args, tt.paramName, tt.defaultValue)
			if got != tt.want {
				t.Errorf("ValidateOptionalNumberParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateBoolParam(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]interface{}
		paramName    string
		defaultValue bool
		want         bool
	}{
		{
			name:         "present true",
			args:         map[string]interface{}{"test": true},
			paramName:    "test",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "present false",
			args:         map[string]interface{}{"test": false},
			paramName:    "test",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "missing parameter",
			args:         map[string]interface{}{},
			paramName:    "test",
			defaultValue: true,
			want:         true,
		},
		{
			name:         "wrong type",
			args:         map[string]interface{}{"test": "true"},
			paramName:    "test",
			defaultValue: false,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateBoolParam(tt.args, tt.paramName, tt.defaultValue)
			if got != tt.want {
				t.Errorf("ValidateBoolParam() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePositiveNumber(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		paramName string
		wantError bool
	}{
		{
			name:      "positive number",
			value:     42.5,
			paramName: "test",
			wantError: false,
		},
		{
			name:      "zero is invalid",
			value:     0,
			paramName: "test",
			wantError: true,
		},
		{
			name:      "negative number",
			value:     -5.0,
			paramName: "test",
			wantError: true,
		},
		{
			name:      "small positive",
			value:     0.001,
			paramName: "test",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotError := ValidatePositiveNumber(tt.value, tt.paramName)

			if (gotError != nil) != tt.wantError {
				t.Errorf("ValidatePositiveNumber() error = %v, wantError %v", gotError != nil, tt.wantError)
			}

			if gotError != nil && !gotError.IsError {
				t.Error("ValidatePositiveNumber() returned response should have IsError = true")
			}
		})
	}
}
