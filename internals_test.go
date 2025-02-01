package germ

import (
	"reflect"
	"testing"
)

func TestUniqueElements(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]string
		expected []string
	}{
		{
			name: "single slice with filesystem paths",
			input: [][]string{
				{"/home/user/docs", "/home/user/images", "/home/user/docs"},
			},
			expected: []string{"/home/user/docs", "/home/user/images"},
		},
		{
			name: "multiple slices with overlapping filesystem paths",
			input: [][]string{
				{"/home/user/docs", "/home/user/images"},
				{"/home/user/images", "/home/user/music"},
			},
			expected: []string{"/home/user/docs", "/home/user/images", "/home/user/music"},
		},
		{
			name:     "empty input with filesystem paths",
			input:    [][]string{{}},
			expected: []string{},
		},
		{
			name: "completely unique filesystem paths",
			input: [][]string{
				{"/var/log/syslog", "/var/log/auth.log"},
				{"/etc/passwd", "/etc/hosts"},
			},
			expected: []string{"/var/log/syslog", "/var/log/auth.log", "/etc/passwd", "/etc/hosts"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := uniqueElements(test.input...)

			if !reflect.DeepEqual(got, test.expected) {
				t.Errorf("uniqueElementsOrdered(%v) = %v; want %v", test.input, got, test.expected)
			}
		})
	}
}
