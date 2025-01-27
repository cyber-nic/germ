package orb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	sitter "github.com/tree-sitter/go-tree-sitter"
	sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// TestNewRepoMap tests the NewRepoMap function.
func TestNewRepoMap(t *testing.T) {
	rm := NewRepoMap(
		1024,
		".",
		&ModelStub{},
		"",
		false,
		16000,
		8,
		"auto",
	)
	if rm == nil {
		t.Fatalf("Expected NewRepoMap to return a non-nil RepoMap")
	}
}

// // TestGetRepoMap tests the GetRepoMap method of the RepoMap struct.
// func TestGetRepoMap(t *testing.T) {
// 	tmpDir, err := os.MkdirTemp("", "repomap-test-*")
// 	if err != nil {
// 		t.Fatalf("Failed to create temp dir: %v", err)
// 	}
// 	defer os.RemoveAll(tmpDir)

// 	// Create two sample files: one for chat, one for "other"
// 	chatFile := filepath.Join(tmpDir, "test_chat.go")
// 	otherFile := filepath.Join(tmpDir, "test_other.go")

// 	err = os.WriteFile(chatFile, []byte(`package main

// func ChatFunc() {
//     // chat code
// }`), 0600)
// 	if err != nil {
// 		t.Fatalf("Failed to create chatFile: %v", err)
// 	}

// 	err = os.WriteFile(otherFile, []byte(`package main

// func OtherFunc() {
//     // other code
// }`), 0600)
// 	if err != nil {
// 		t.Fatalf("Failed to create otherFile: %v", err)
// 	}

// 	rm := NewRepoMap(
// 		1024,
// 		tmpDir,
// 		&ModelStub{},
// 		"",
// 		false,
// 		16000,
// 		8,
// 		"auto",
// 	)

// 	chatFiles := []string{chatFile}
// 	repoFiles := []string{otherFile}
// 	mentionedFnames := map[string]bool{}
// 	mentionedIdents := map[string]bool{}
// 	forceRefresh := false

// 	repoMapText := rm.GetRepoMap(chatFiles, repoFiles, mentionedFnames, mentionedIdents, forceRefresh)
// 	if len(repoMapText) == 0 {
// 		t.Errorf("Expected GetRepoMap to return non-empty text, got empty string")
// 	}
// }

// // TestGetRankedTagsMap tests the GetRankedTagsMap method of the RepoMap struct.
// func TestGetRankedTagsMap(t *testing.T) {
// 	tmpDir, err := os.MkdirTemp("", "repomap-test-*")
// 	if err != nil {
// 		t.Fatalf("Failed to create temp dir: %v", err)
// 	}
// 	defer os.RemoveAll(tmpDir)

// 	fileA := filepath.Join(tmpDir, "a.go")
// 	contentA := `package main

// func A() {
//     // some code
// }
// `
// 	if err := os.WriteFile(fileA, []byte(contentA), 0600); err != nil {
// 		t.Fatalf("Failed to create fileA: %v", err)
// 	}

// 	fileB := filepath.Join(tmpDir, "b.go")
// 	contentB := `package main

// func B() {
//     // some other code
// }
// `
// 	if err := os.WriteFile(fileB, []byte(contentB), 0600); err != nil {
// 		t.Fatalf("Failed to create fileB: %v", err)
// 	}

// 	rm := NewRepoMap(
// 		1024,
// 		tmpDir,
// 		&ModelStub{},
// 		"",
// 		false,
// 		16000,
// 		8,
// 		"auto",
// 	)

// 	chatFiles := []string{fileA}
// 	otherFiles := []string{fileB}
// 	mentionedFnames := map[string]bool{}
// 	mentionedIdents := map[string]bool{}
// 	forceRefresh := false

// 	rankedMap := rm.GetRankedTagsMap(chatFiles, otherFiles, 1024, mentionedFnames, mentionedIdents, forceRefresh)
// 	if len(rankedMap) == 0 {
// 		t.Errorf("Expected GetRankedTagsMap to return non-empty text, got empty string")
// 	}
// }

// TestRenderTree tests the renderTree method of the RepoMap struct.
func TestRenderTree(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repomap-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sampleFile := filepath.Join(tmpDir, "demo.go")
	// This file has 5 lines total (0..4), so note the line indexes carefully:
	// Line 0: package main
	// Line 1:
	// Line 2: func Demo() {
	// Line 3:     // Some line of code
	// Line 4: }
	err = os.WriteFile(sampleFile, []byte(`package main

func Demo() {
    // Some line of code
}
`), 0600)
	if err != nil {
		t.Fatalf("Failed to create sample file: %v", err)
	}

	rm := NewRepoMap(
		1024,
		tmpDir,
		&ModelStub{},
		"",
		false,
		16000,
		8,
		"auto",
	)

	// If we want to see the line "// Some line of code," that's on line 3 (0-based).
	linesOfInterest := []int{2, 3}
	rendered, _ := rm.renderTree(sampleFile, "demo.go", linesOfInterest)

	if !strings.Contains(rendered, "func Demo()") {
		t.Errorf("Expected snippet to contain 'func Demo()', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Some line of code") {
		t.Errorf("Expected snippet to contain 'Some line of code', got:\n%s", rendered)
	}
}

// TestGetRelFname tests the GetRelFname method of the RepoMap struct.
func TestGetRelFname(t *testing.T) {
	// Test cases
	tests := []struct {
		name     string
		root     string
		fname    string
		expected string
	}{
		{
			name:     "File within root",
			root:     "/home/user/project",
			fname:    "/home/user/project/file.txt",
			expected: "file.txt",
		},
		{
			name:     "Nested file within root",
			root:     "/home/user/project",
			fname:    "/home/user/project/folder/file.txt",
			expected: "folder/file.txt",
		},
		{
			name:     "File outside root",
			root:     "/home/user/project",
			fname:    "/home/user/other/file.txt",
			expected: "../other/file.txt",
		},
		{
			name:     "Same as root",
			root:     "/home/user/project",
			fname:    "/home/user/project",
			expected: ".",
		},
		{
			name:     "Empty root",
			root:     "",
			fname:    "/home/user/project/file.txt",
			expected: "/home/user/project/file.txt",
		},
		{
			name:     "Empty fname returns as-is",
			root:     "/home/user/project",
			fname:    ".",
			expected: ".",
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := RepoMap{Root: tt.root}
			result := repo.GetRelFname(tt.fname)

			if result != tt.expected {
				t.Errorf("GetRelFname(%q) = %q; want %q", tt.fname, result, tt.expected)
			}
		})
	}
}

// TestGetSourceCodeMapQuery tests the getSourceCodeMapQuery method of the RepoMap struct.
func TestGetSourceCodeMapQuery(t *testing.T) {
	// Initialize RepoMap
	repo := RepoMap{
		querySourceCache: make(map[string]string),
	}

	// Test cases
	tests := []struct {
		name          string
		lang          string
		expectedError bool
		expectedData  string
	}{
		{
			name:          "Valid language: Go",
			lang:          "go",
			expectedError: false,
		},
		{
			name:          "Valid language: Python",
			lang:          "python",
			expectedError: false,
		},
		{
			name:          "Valid language: JavaScript",
			lang:          "javascript",
			expectedError: false,
		},
		{
			name:          "Unsupported language",
			lang:          "haskell",
			expectedError: true,
		},
		{
			name:          "Cached query source",
			lang:          "go",
			expectedError: false,
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			data, err := repo.getSourceCodeMapQuery(tt.lang)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				// validate query source
				tpl := "queries/tree-sitter-%s-tags.scm"
				queryFilename := fmt.Sprintf(tpl, tt.lang)

				// read query file for validation
				querySource, err := os.ReadFile(queryFilename)
				if err != nil {
					t.Errorf("failed to read file %q: %v", queryFilename, err)
				}

				if data != string(querySource) {
					t.Errorf("expected data %q, got %q", string(querySource), data)
				}
			}
		})
	}
}

// TestGetTagsFromQueryCapture_EmptySource verifies that providing an empty
// source code snippet returns no tags.
func TestGetTagsFromQueryCapture_EmptySource(t *testing.T) {
	// An empty source code snippet
	sourceCode := []byte("package main")

	// Create a language object
	lang := sitter.NewLanguage(sitter_go.Language())

	// Get source code CST
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree := parser.Parse(sourceCode, nil)

	// Use a trivial, empty query. This won't match anything.
	q, err := sitter.NewQuery(lang, "")
	if err != nil {
		t.Fatalf("Failed to create an empty query: %v", err)
	}

	// Invoke the function under test
	tags := GetTagsFromQueryCapture("rel/path.go", "/absolute/path.go", q, tree, sourceCode, nil)

	// We expect no tags
	assert.Empty(t, tags, "Expected no tags for empty source code")
}

// TestGetTagsFromQueryCapture_SimpleDef simulates a scenario with a minimal
// snippet containing a single "definition-like" capture.
func TestGetTagsFromQueryCapture_SimpleDef(t *testing.T) {
	// Fake snippet that could trigger a "definition" capture in an actual grammar.
	sourceCode := []byte("package main; func Hello() {}")

	// Create a language object
	lang := sitter.NewLanguage(sitter_go.Language())

	// Get source code CST
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree := parser.Parse(sourceCode, nil)

	// go function definition query
	query := `
	(
	  (function_declaration name: (identifier) @name.definition.function) @definition.function
	)
	`
	// Create the query
	q, err := sitter.NewQuery(lang, query)
	if err != nil {
		t.Fatalf("Failed to create query: %v", err)
	}

	// Call the function we want to test
	tags := GetTagsFromQueryCapture("rel/path.go", "/absolute/path.go", q, tree, sourceCode, nil)

	// Example: we expect exactly one definition. Adjust to reality once
	// your parser/query code is set up.
	assert.Len(t, tags, 1, "Expected exactly one definition capture")
	assert.Equal(t, TagKindDef, tags[0].Kind, "Expected the capture to be recognized as a definition")
	// Hypothetical name check
	assert.Equal(t, "Hello", tags[0].Name, "Expected the function name to match 'Hello'")
}

// TestGetTagsFromQueryCapture_SimpleRef simulates a scenario with a snippet
// containing a single "reference-like" capture.
func TestGetTagsFromQueryCapture_SimpleRef(t *testing.T) {
	// Fake snippet that might trigger a "reference" capture (function call, etc.).
	sourceCode := []byte(`
	import "fmt"

	func hello() {
		fmt.Println("Hello")
	}

	func main() {
		hello()
		hello()
	}
	`)

	// Create a language object
	lang := sitter.NewLanguage(sitter_go.Language())

	// Get source code CST
	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree := parser.Parse(sourceCode, nil)

	// call expression query
	query := `
	(
	  (call_expression function: (identifier) @name.reference.call) @reference.call
	)
	`
	q, err := sitter.NewQuery(lang, query)
	if err != nil {
		t.Fatalf("Failed to create query: %v", err)
	}

	// Invoke the function
	tags := GetTagsFromQueryCapture("rel/path.go", "/absolute/path.go", q, tree, sourceCode, nil)

	assert.Len(t, tags, 2, "Expected exactly one reference capture")
	assert.Equal(t, TagKindRef, tags[0].Kind, "Expected the capture to be recognized as a reference")
	assert.True(t, strings.Contains(tags[0].Name, "hello"), "Expected reference to contain 'hello'")
	assert.Equal(t, tags[1].Line, 9, "Expected reference to be on line 9")
}
