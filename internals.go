package orb

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// PrintStruct prints a struct as JSON.
func PrintStruct(w io.Writer, t interface{}) {
	j, _ := json.MarshalIndent(t, "", "  ")
	fmt.Fprintln(w, string(j))
}

func PrintStructOut(t interface{}) {
	PrintStruct(os.Stdout, t)
}

// uniqueElements combines two or more string slices and returns a map of unique unique elements.
func uniqueElements(slices ...[]string) map[string]struct{} {
	// Initialize a map to track unique elements
	uniqueMap := make(map[string]struct{})

	// Iterate over all provided slices
	for _, slice := range slices {
		for _, elem := range slice {
			uniqueMap[elem] = struct{}{}
		}
	}

	return uniqueMap
}

// filterImportantFiles is a stub to mimic Python's `filter_important_files`.
func filterImportantFiles(files []string) []string {
	return files
}

// Simple in-place sort for Tag slices by a custom comparator (used in getRankedTags).
func simpleSort(tags []Tag, lessFn func(a, b Tag) bool) {
	if len(tags) < 2 {
		return
	}
	quickSort(tags, 0, len(tags)-1, lessFn)
}

func quickSort(tags []Tag, left, right int, lessFn func(a, b Tag) bool) {
	if left >= right {
		return
	}
	pivot := partition(tags, left, right, lessFn)
	quickSort(tags, left, pivot-1, lessFn)
	quickSort(tags, pivot+1, right, lessFn)
}

func partition(tags []Tag, left, right int, lessFn func(a, b Tag) bool) int {
	pivot := tags[right]
	i := left
	for j := left; j < right; j++ {
		if lessFn(tags[j], pivot) {
			tags[i], tags[j] = tags[j], tags[i]
			i++
		}
	}
	tags[i], tags[right] = tags[right], tags[i]
	return i
}
