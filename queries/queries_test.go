package scm

import (
	"testing"
)

func TestGetSitterQuery(t *testing.T) {
	tests := []struct {
		name      string
		language  SitterLanguage
		wantQuery []byte
		wantErr   bool
	}{
		{
			name:      "valid language CSharp",
			language:  CSharp,
			wantQuery: cSharpTagQuery,
			wantErr:   false,
		},
		{
			name:      "valid language Go",
			language:  Go,
			wantQuery: goTagQuery,
			wantErr:   false,
		},
		{
			name:      "invalid language",
			language:  "invalid",
			wantQuery: []byte{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, err := GetSitterQuery(tt.language)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSitterQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// compare byte slices
			if string(gotQuery) != string(tt.wantQuery) {
				t.Errorf("GetSitterQuery() = %v, want %v", gotQuery, tt.wantQuery)
			}
		})
	}
}
