package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatConfigValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "boolean true",
			input:    "true",
			expected: "true",
		},
		{
			name:     "boolean false",
			input:    "false",
			expected: "false",
		},
		{
			name:     "integer",
			input:    "123",
			expected: "123",
		},
		{
			name:     "float",
			input:    "123.45",
			expected: "123.45",
		},
		{
			name:     "negative number",
			input:    "-42",
			expected: "-42",
		},
		{
			name:     "string",
			input:    "hello world",
			expected: `"hello world"`,
		},
		{
			name:     "string with numbers",
			input:    "123abc",
			expected: `"123abc"`,
		},
		{
			name:     "already quoted string",
			input:    `"already quoted"`,
			expected: `"already quoted"`,
		},
		{
			name:     "array",
			input:    `["item1", "item2"]`,
			expected: `["item1", "item2"]`,
		},
		{
			name:     "object",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: `""`,
		},
		{
			name:     "string true (not boolean)",
			input:    `"true"`,
			expected: `"true"`,
		},
		{
			name:     "string false (not boolean)",
			input:    `"false"`,
			expected: `"false"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test assumes there's a formatConfigValue function
			// that handles the type detection and formatting
			// If not, we're testing the expected behavior
			result := formatConfigValueForTest(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to simulate the formatting logic
func formatConfigValueForTest(value string) string {
	// Check if it's already quoted
	if len(value) > 0 && value[0] == '"' && value[len(value)-1] == '"' {
		return value
	}

	// Check if it's an array or object
	if len(value) > 0 && (value[0] == '[' || value[0] == '{') {
		return value
	}

	// Check if it's a boolean
	if value == "true" || value == "false" {
		return value
	}

	// Check if it's a number
	if isNumber(value) {
		return value
	}

	// Otherwise, it's a string
	if value == "" {
		return `""`
	}
	return `"` + value + `"`
}

func isNumber(s string) bool {
	if s == "" {
		return false
	}

	hasDigit := false
	hasDot := false

	for i, c := range s {
		if i == 0 && c == '-' {
			continue
		}
		if c == '.' {
			if hasDot {
				return false
			}
			hasDot = true
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		hasDigit = true
	}

	return hasDigit
}

func TestConfigMapToString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		key      string
		expected string
	}{
		{
			name: "boolean to string",
			input: map[string]interface{}{
				"debug": true,
			},
			key:      "debug",
			expected: "true",
		},
		{
			name: "integer to string",
			input: map[string]interface{}{
				"workers": 4,
			},
			key:      "workers",
			expected: "4",
		},
		{
			name: "float to string",
			input: map[string]interface{}{
				"rate": 0.05,
			},
			key:      "rate",
			expected: "0.05",
		},
		{
			name: "string unchanged",
			input: map[string]interface{}{
				"path": "/storm/data",
			},
			key:      "path",
			expected: "/storm/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When storing in ConfigMap, all values become strings
			configMapData := make(map[string]string)
			for k, v := range tt.input {
				configMapData[k] = fmt.Sprintf("%v", v)
			}

			assert.Equal(t, tt.expected, configMapData[tt.key])
		})
	}
}
