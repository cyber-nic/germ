package orb

import (
	"reflect"
	"testing"
)

func TestUniqueElements(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]string
		expected map[string]struct{}
	}{
		{
			name:     "No input slices",
			input:    [][]string{},
			expected: map[string]struct{}{},
		},
		{
			name:     "Single slice with unique elements",
			input:    [][]string{{"a", "b", "c"}},
			expected: map[string]struct{}{"a": {}, "b": {}, "c": {}},
		},
		{
			name:     "Two slices with overlapping elements",
			input:    [][]string{{"a", "b", "c"}, {"b", "c", "d"}},
			expected: map[string]struct{}{"a": {}, "b": {}, "c": {}, "d": {}},
		},
		{
			name:     "Multiple slices with some empty",
			input:    [][]string{{"a", "b"}, {}, {"c", "a"}},
			expected: map[string]struct{}{"a": {}, "b": {}, "c": {}},
		},
		{
			name:     "All slices are empty",
			input:    [][]string{{}, {}},
			expected: map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueElements(tt.input...)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("UniqueElements() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
