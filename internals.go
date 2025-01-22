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
