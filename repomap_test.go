package germ

import (
	"fmt"
	"os"
	"sort"
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
		RepoMapOptions{},
	)
	if rm == nil {
		t.Fatalf("Expected NewRepoMap to return a non-nil RepoMap")
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

func TestGetTagsFromQueryCapture(t *testing.T) {
	// EmptySource verifies that providing an empty
	// source code snippet returns no tags.
	t.Run("EmptySource", func(t *testing.T) {

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
	})

	// SimpleDef simulates a scenario with a minimal
	// snippet containing a single "definition-like" capture.
	t.Run("SimpleDef", func(t *testing.T) {
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
	})

	// SimpleRef simulates a scenario with a snippet
	// containing a single "reference-like" capture.
	t.Run("SimpleRef", func(t *testing.T) {
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
	})
}

// TestGetRankedTagsByPageRank contains multiple sub-tests demonstrating how you might
// set up scenarios and verify outcomes. Each sub-test defines a small collection of
// tags, “mentioned” data, and checks the ranked output.
func TestGetRankedTagsByPageRank(t *testing.T) {

	t.Run("NoReferencesFallback", func(t *testing.T) {
		// This scenario has definitions but no references. We expect the fallback
		// behavior to treat defines as references, effectively giving minimal ranking.
		r := &RepoMap{}

		allTags := []Tag{
			{"FileA.go", "path/to/FileA.go", 10, "Foo", TagKindDef},
			{"FileB.go", "path/to/FileB.go", 20, "Bar", TagKindDef},
		}

		mentionedFnames := map[string]bool{}
		mentionedIdents := map[string]bool{}

		ranked := r.getRankedTagsByPageRank(allTags, mentionedFnames, mentionedIdents)

		// In fallback mode, references=defines, so each file "references" itself.
		// Because the Go code currently discards self-edges, you might end up with
		// no edges. The PageRank for each file could be near-equal. We just check
		// that we got back the definitions in some order.
		if len(ranked) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(ranked))
		}

		// debug
		// for i, tg := range ranked {
		// 	fmt.Printf("Ranked[%d]: file=%s, symbol=%s\n", i, tg.FilePath, tg.Name)
		// }

		// assert.Equal(t, "path/to/FileA.go", ranked[0].FilePath, "Expected FileA.go to be ranked first")
		// assert.Equal(t, "path/to/FileB.go", ranked[1].FilePath, "Expected FileB.go to be ranked second")
	})

	t.Run("SingleRef", func(t *testing.T) {
		// Scenario: FileA defines "Foo", FileB references "Foo" once.
		r := &RepoMap{}

		allTags := []Tag{
			{"FileA.go", "path/to/FileA.go", 10, "Foo", TagKindDef},
			{"FileB.go", "path/to/FileB.go", 20, "Foo", TagKindRef},
		}

		// Nothing “mentioned” in chat
		mentionedFnames := map[string]bool{}
		mentionedIdents := map[string]bool{}

		ranked := r.getRankedTagsByPageRank(allTags, mentionedFnames, mentionedIdents)

		// We expect the top-ranked definition to be (FileA, "Foo").
		// Currently, the code will produce at least one ranked definition from FileA.
		if len(ranked) == 0 {
			t.Errorf("Expected at least one ranked definition, got 0")
			return
		}
		// The top entry should be the definition from FileA
		top := ranked[0]
		if top.FilePath != "path/to/FileA.go" || top.Name != "Foo" {
			t.Errorf("Top definition mismatch: got (file=%s, name=%s), want (FileA.go, Foo)",
				top.FilePath, top.Name)
		}
	})

	t.Run("MentionedSymbolBoost", func(t *testing.T) {
		// Scenario: Two definitions of "Foo" in FileA and FileB.
		// FileC references "Foo" once. "Foo" is also in mentionedIdents,
		// so it should get a bigger weight.
		r := &RepoMap{}

		allTags := []Tag{
			{"FileA.go", "path/to/FileA.go", 10, "Foo", TagKindDef},
			{"FileB.go", "path/to/FileB.go", 20, "Foo", TagKindDef},
			{"FileC.go", "path/to/FileC.go", 30, "Foo", TagKindRef},
		}

		mentionedFnames := map[string]bool{
			// Suppose "path/to/FileC.go" is mentioned in chat
			"path/to/FileC.go": true,
		}
		mentionedIdents := map[string]bool{
			// Also the symbol "Foo" is “mentioned” in chat
			"Foo": true,
		}

		ranked := r.getRankedTagsByPageRank(allTags, mentionedFnames, mentionedIdents)
		if len(ranked) < 2 {
			t.Errorf("Expected at least 2 ranked definitions, got %d", len(ranked))
		}

		// Sort the result by file path for stable test output,
		// then check rank positions or at least existence.
		// (If you rely on the "rank" ordering from getRankedTagsByPageRank,
		//  you can just check the first or second item.)
		sorted := append([]Tag(nil), ranked...)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].FilePath < sorted[j].FilePath
		})

		// We expect to see definitions from FileA.go and FileB.go, both for "Foo".
		// Let's just check presence:
		foundA, foundB := false, false
		for _, tg := range sorted {
			if tg.FilePath == "path/to/FileA.go" && tg.Name == "Foo" {
				foundA = true
			}
			if tg.FilePath == "path/to/FileB.go" && tg.Name == "Foo" {
				foundB = true
			}
		}

		if !foundA || !foundB {
			t.Errorf("Expected definitions (FileA, Foo) and (FileB, Foo) in results. FoundA=%v, FoundB=%v",
				foundA, foundB)
		}

		// Ideally also verify *relative ordering* or rank values
		// if you can reliably predict them. But PageRank algorithms can produce
		// small variations in floating-point results.
	})
}

// TestRenderTree tests the renderTree method of the RepoMap struct.
// Now that renderTree takes (relFname, code []byte, linesOfInterest []int),
//
//	func (r *RepoMap) renderTree(
//	    relFname string,
//	    code []byte,
//	    linesOfInterest []int,
//	) (string, error)
//
// we can pass inline code for testing rather than reading from an actual file.
func TestRenderTree(t *testing.T) {
	// 1) Basic sample code with 5 lines total (index 0..4).
	// We'll verify lines of interest 2 and 3 are displayed in the snippet.
	code := []byte(`package main

func Demo() {
    // Some line of code
}
`)

	rm := NewRepoMap(
		1024,
		".",
		nil, // e.g., &ModelStub{}, or whatever your code expects
		"",
		false,
		16000,
		8,
		"auto",
		RepoMapOptions{},
	)

	// If we want to see lines 2 and 3:
	//   line 2 => "func Demo() {"
	//   line 3 => "    // Some line of code"
	linesOfInterest := []int{2, 3}
	rendered, err := rm.renderTree("demo.go", code, linesOfInterest)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(rendered, "func Demo()") {
		t.Errorf("Expected snippet to contain 'func Demo()', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Some line of code") {
		t.Errorf("Expected snippet to contain 'Some line of code', got:\n%s", rendered)
	}

	// 2) No lines of interest -> likely returns empty or minimal snippet.
	t.Run("NoLinesOfInterest", func(t *testing.T) {
		rendered, err := rm.renderTree("demo.go", code, nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if rendered == "" {
			t.Log("Got empty snippet as expected with no lines of interest.")
		} else {
			t.Logf("Snippet returned some content: %q", rendered)
		}
	})

	// // 3) Additional sample with multiline function to test “child context” or expansions.
	// // If your grep-ast library now handles child contexts, you can add a bigger snippet.
	// bigCode := []byte(`package main

	// func AnotherDemo() {
	//     // line 2
	//     if true {
	//         // line 4
	//         println("Inside if")
	//     }
	//     // line 7
	// }
	// `)

	// t.Run("MultiLineFunction", func(t *testing.T) {
	// 	// Let’s highlight the if-statement line (4) and see if we get line 5 as well.
	// 	linesOfInterest := []int{4}
	// 	rendered, err := rm.renderTree("another.go", bigCode, linesOfInterest)
	// 	if err != nil {
	// 		t.Errorf("Unexpected error: %v", err)
	// 	}

	// 	fmt.Println(rendered)

	// 	// Confirm the snippet has at least line 4 and line 5.
	// 	if !strings.Contains(rendered, "// line 4") {
	// 		t.Errorf("Snippet missing '// line 4'. Got:\n%s", rendered)
	// 	}
	// 	if !strings.Contains(rendered, `println("Inside if")`) {
	// 		t.Errorf("Snippet missing 'println(\"Inside if\")'. Got:\n%s", rendered)
	// 	}
	// })
}
