package orb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestGetRepoMap(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repomap-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create two sample files: one for chat, one for "other"
	chatFile := filepath.Join(tmpDir, "test_chat.go")
	otherFile := filepath.Join(tmpDir, "test_other.go")

	err = os.WriteFile(chatFile, []byte(`package main

func ChatFunc() {
    // chat code
}`), 0600)
	if err != nil {
		t.Fatalf("Failed to create chatFile: %v", err)
	}

	err = os.WriteFile(otherFile, []byte(`package main

func OtherFunc() {
    // other code
}`), 0600)
	if err != nil {
		t.Fatalf("Failed to create otherFile: %v", err)
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

	chatFiles := []string{chatFile}
	otherFiles := []string{otherFile}
	mentionedFnames := map[string]bool{}
	mentionedIdents := map[string]bool{}
	forceRefresh := false

	repoMapText := rm.GetRepoMap(chatFiles, otherFiles, mentionedFnames, mentionedIdents, forceRefresh)
	if len(repoMapText) == 0 {
		t.Errorf("Expected GetRepoMap to return non-empty text, got empty string")
	}
}

func TestGetRankedTagsMap(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "repomap-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	fileA := filepath.Join(tmpDir, "a.go")
	contentA := `package main

func A() {
    // some code
}
`
	if err := os.WriteFile(fileA, []byte(contentA), 0600); err != nil {
		t.Fatalf("Failed to create fileA: %v", err)
	}

	fileB := filepath.Join(tmpDir, "b.go")
	contentB := `package main

func B() {
    // some other code
}
`
	if err := os.WriteFile(fileB, []byte(contentB), 0600); err != nil {
		t.Fatalf("Failed to create fileB: %v", err)
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

	chatFiles := []string{fileA}
	otherFiles := []string{fileB}
	mentionedFnames := map[string]bool{}
	mentionedIdents := map[string]bool{}
	forceRefresh := false

	rankedMap := rm.GetRankedTagsMap(chatFiles, otherFiles, 1024, mentionedFnames, mentionedIdents, forceRefresh)
	if len(rankedMap) == 0 {
		t.Errorf("Expected GetRankedTagsMap to return non-empty text, got empty string")
	}
}

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
	rendered := rm.renderTree(sampleFile, "demo.go", linesOfInterest)

	if !strings.Contains(rendered, "func Demo()") {
		t.Errorf("Expected snippet to contain 'func Demo()', got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Some line of code") {
		t.Errorf("Expected snippet to contain 'Some line of code', got:\n%s", rendered)
	}
}
