package main

import (
	"testing"
)

func TestIsAlphaNumericString(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"", true},
		{"abcdefghijklmnopqrstuvwxyz", true},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZ", true},
		{"1234567890", true},
		{"abc123", true},
		{"abc_123", false},
		{"abc 123", false},
		{"abc-123", false},
	}

	for _, tc := range testCases {
		actual := isAlphaNumericString(tc.input)
		if actual != tc.expected {
			t.Errorf("isAlphaNumericString(%q): expected %v, got %v", tc.input, tc.expected, actual)
		}
	}
}
