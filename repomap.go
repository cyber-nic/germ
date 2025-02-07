package germ

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	// Import the grep-ast library
	scm "github.com/cyber-nic/germ/scm"
	goignore "github.com/cyber-nic/go-gitignore"
	grepast "github.com/cyber-nic/grep-ast"
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/rs/zerolog/log"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/multi"
	"gonum.org/v1/gonum/graph/network"
)

// chat files: files that are part of the chat
// other files: files that are not (yet) part of the chat
// warned_files: files that have been warned about and are excluded

const (
	TagKindDef = "def"
	TagKindRef = "ref"
)

// Tag represents a “tag” extracted from a source file.
type Tag struct {
	FileName string
	FilePath string
	Line     int
	Name     string
	Kind     string
}

var (
	ErrOperational = errors.New("operational error")
	ErrDatabase    = errors.New("database error")
)

// RepoMap is the Go equivalent of the Python class `RepoMap`.
type RepoMap struct {
	Refresh           string
	Verbose           bool
	Root              string
	MainModel         *ModelStub
	RepoContentPx     string
	MaxMapTokens      int
	MaxCtxWindow      int
	MapMulNoFiles     int
	MapProcessingTime float64
	LastMap           string
}

// ModelStub simulates the main_model used in Python code (for token_count, etc.).
type ModelStub struct{}

// TokenCount is a naive token estimator. Real code might call tiktoken or other logic.
func (m *ModelStub) TokenCount(text string) int {
	// Very naive: 1 token ~ 4 chars
	return len(text) / 4
}

// ------------------------------------------------------------------------------------
// RepoMap Constructor
// ------------------------------------------------------------------------------------
func NewRepoMap(
	maxMapTokens int,
	root string,
	mainModel *ModelStub,
	repoContentPrefix string,
	verbose bool,
	maxContextWindow int,
	mapMulNoFiles int,
	refresh string,
	options RepoMapOptions,
) *RepoMap {
	if root == "" {
		cwd, err := os.Getwd()
		if err == nil {
			root = cwd
		}
	}

	r := &RepoMap{
		Refresh:       refresh,
		Verbose:       verbose,
		Root:          root,
		MainModel:     mainModel,
		RepoContentPx: repoContentPrefix,
		MaxMapTokens:  maxMapTokens,
		MapMulNoFiles: mapMulNoFiles,
		MaxCtxWindow:  maxContextWindow,
	}

	if verbose {
		fmt.Printf("RepoMap initialized with map_mul_no_files: %d\n", mapMulNoFiles)
	}
	return r
}

// GetRelFname returns fname relative to r.Root. If that fails, returns fname as-is.
func (r *RepoMap) GetRelFname(fname string) string {
	rel, err := filepath.Rel(r.Root, fname)
	if err != nil {
		return fname
	}

	return rel
}

// TokenCount tries to mimic how the Python code estimates tokens (split into short vs. large).
func (r *RepoMap) TokenCount(text string) float64 {
	if len(text) < 200 {
		return float64(r.MainModel.TokenCount(text))
	}

	lines := strings.SplitAfter(text, "\n")
	numLines := len(lines)
	step := numLines / 100
	if step < 1 {
		step = 1
	}
	var sb strings.Builder
	for i := 0; i < numLines; i += step {
		sb.WriteString(lines[i])
	}
	sampleText := sb.String()
	sampleTokens := float64(r.MainModel.TokenCount(sampleText))
	ratio := sampleTokens / float64(len(sampleText))
	return ratio * float64(len(text))
}

// GetFileTags calls GetTagsRaw and filters out short names and common words.
func (r *RepoMap) GetFileTags(fname, relFname string, filter TagFilter) ([]Tag, error) {

	// Not cached or changed; re-parse
	data, err := r.GetTagsRaw(fname, relFname, filter)
	if err != nil {
		return nil, err
	}

	if data == nil {
		data = nil
	}

	return data, nil
}

// LoadQuery loads the Tree-sitter query text and compiles a sitter.Query.
func (r *RepoMap) LoadQuery(lang *sitter.Language, langID string) (*sitter.Query, error) {
	querySource, err := scm.GetSitterQuery(scm.SitterLanguage(langID))
	if err != nil {
		return nil, fmt.Errorf("failed to obtain query (%s): %w", langID, err)
	}
	if len(querySource) == 0 {
		return nil, fmt.Errorf("empty query file: %s", langID)
	}

	q, qErr := sitter.NewQuery(lang, querySource)
	if qErr != nil {
		var queryErr *sitter.QueryError
		if errors.As(qErr, &queryErr) {
			if queryErr != nil {
				return nil, fmt.Errorf(
					"query error: %s at row: %d, column: %d, offset: %d, kind: %v",
					queryErr.Message, queryErr.Row, queryErr.Column, queryErr.Offset, queryErr.Kind,
				)
			}
			return nil, fmt.Errorf("unexpected nil *sitter.QueryError")
		}
		return nil, fmt.Errorf("failed to create query: %w", qErr)
	}
	return q, nil
}

func ReadSourceCode(fname string) ([]byte, error) {
	sourceCode, err := os.ReadFile(fname)
	if err != nil {
		return nil, fmt.Errorf("failed to read file (%s): %w", fname, err)
	}
	if len(sourceCode) == 0 {
		return nil, fmt.Errorf("empty file: %s", fname)
	}
	return sourceCode, nil
}

type TagFilter func(name string) bool

// GetTagsFromQueryCapture extracts tags from the result
// of a Tree-sitter query on a given file. It iterates through
// the captures returned by the Tree-sitter query cursor and collects
// definitions (def) and references (ref). All other captures are ignored.
// filter is a function that accepts the name of a capture and returns bool false if it should be skipped.
func GetTagsFromQueryCapture(relFname, fname string, q *sitter.Query, tree *sitter.Tree, sourceCode []byte, filter TagFilter) []Tag {

	// Create a new query cursor that will be used to iterate through
	// the captures of our query on the provided parse tree. The query
	// cursor manages iteration state for match captures.
	qc := sitter.NewQueryCursor()
	defer qc.Close()

	// Execute the query against the provided parse tree, specifying the
	// source code as well. The captures method returns a Captures object
	// which allows iteration over matched captures in the parse tree.
	captures := qc.Captures(q, tree.RootNode(), sourceCode)

	tags := []Tag{}

	// Iterate over all of the query results (i.e., the captures). The Next
	// method returns a matched result (match) and the index of the capture
	// (index) within that match. Continue iterating until match is nil.
	for match, index := captures.Next(); match != nil; match, index = captures.Next() {

		// Retrieve the capture at the current index from the match's list
		// of captures. This capture includes the node in the AST and the
		// capture index used to look up the capture name.
		c := match.Captures[index]

		// Retrieve the name of the capture using the capture index stored in
		// c.Index. This references the actual capture label (e.g.,
		// "name.definition.function") in the query's capture names.
		tag := q.CaptureNames()[c.Index]

		// Convert the node's starting row position to an integer (ie. line number)
		row := int(c.Node.StartPosition().Row)

		// Extract the raw text from the matched node in the source code. We
		// convert it from a slice of bytes to a string.
		name := string(c.Node.Utf8Text(sourceCode))

		// Allows a user-provided list of terms to skip: eg. bool, string, etc.
		if filter != nil && !filter(name) {
			continue
		}

		// Determine if the capture corresponds to a definition or a reference
		// by checking prefixes in its name. If neither condition matches, we
		// skip it.
		switch {
		case strings.HasPrefix(tag, "name.definition."):
			// eg. function, method, type, etc.
			tags = append(tags, Tag{
				Name:     name,
				FileName: relFname,
				FilePath: fname,
				Line:     row,
				Kind:     TagKindDef,
			})

		case strings.HasPrefix(tag, "name.reference."):
			//eg. function call, type usage, etc.
			tags = append(tags, Tag{
				Name:     name,
				FileName: relFname,
				FilePath: fname,
				Line:     row,
				Kind:     TagKindRef,
			})

		default:
			// continue
		}
	}

	return tags
}

// GetTagsRaw parses the file with Tree-sitter and extracts "function definitions"
func (r *RepoMap) GetTagsRaw(fname, relFname string, filter TagFilter) ([]Tag, error) {
	// 1) Identify the file's language
	lang, langID, err := grepast.GetLanguageFromFileName(fname)
	if err != nil || lang == nil {
		return nil, grepast.ErrorUnsupportedLanguage
	}

	// 2) Read source code
	sourceCode, err := ReadSourceCode(fname)
	if err != nil {
		return nil, fmt.Errorf("failed to read file (%s): %v", fname, err)
	}

	// 3) Create parser
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	// 4) Parse
	tree := parser.Parse(sourceCode, nil)
	if tree == nil || tree.RootNode() == nil {
		return nil, fmt.Errorf("failed to parse file: %s", fname)
	}

	// 5) Load your query
	q, err := r.LoadQuery(lang, langID)
	if err != nil {
		return nil, fmt.Errorf("failed to read query file (%s): %v", langID, err)
	}
	defer q.Close()

	// 6) Execute the query
	qc := sitter.NewQueryCursor()
	defer qc.Close()

	// Get the tags from the query capture and source code
	tags := GetTagsFromQueryCapture(relFname, fname, q, tree, sourceCode, filter)

	// 7) Return the list of Tag objects
	return tags, nil
}

// getTagsFromFiles collect all tags from those files
func (r *RepoMap) getTagsFromFiles(allFnames []string, ignoreWords map[string]struct{}) []Tag {

	var allTags []Tag

	for _, fname := range allFnames {
		// Get the relative file name
		rel := r.GetRelFname(fname)

		// Filter out short names and common words
		// tr@ck - where is the right place to put this filter?
		filter := func(name string) bool {
			if len(name) <= 2 {
				return false
			}
			if _, ok := ignoreWords[strings.ToLower(name)]; ok {
				return false
			}
			return true
		}

		// Get the tags for this file
		tg, err := r.GetFileTags(fname, rel, filter)
		if err != nil {
			if err == grepast.ErrorUnsupportedLanguage {
				log.Trace().Msgf("skip %s", fname)
			} else {
				log.Warn().Err(err).Msgf("Failed to get tags for %s", fname)
			}
			continue
		}

		// ndelorme - file tags
		// fmt.Println("Tags for file:", fname)
		// for _, t := range tg {
		// 	fmt.Printf("- %s / %d / %s\n", t.Kind, t.Line, t.Name)
		// }

		if tg != nil {
			allTags = append(allTags, tg...)
		}
	}

	return allTags
}

type tagKey struct {
	fname  string // the file name (relative)
	symbol string // the actual identifier
}

func (r *RepoMap) getRankedTagsByPageRank(allTags []Tag, mentionedFnames, mentionedIdents map[string]bool) []Tag {

	//--------------------------------------------------------
	// 1) Build up references/defines data structures
	//--------------------------------------------------------
	defines, references, definitions, identifiers := r.buildReferenceMaps(allTags)

	if r.Verbose {
		// ndelorme
		fmt.Printf("\n\n## defines:")
		for k, v := range defines {
			fmt.Printf("\n- %s: %v", k, v)
		}
		fmt.Printf("\n\n## definitions:")
		for k, v := range definitions {
			fmt.Printf("\n- %s: %v", k, v)
		}
		fmt.Printf("\n\n## references:")
		for k, v := range references {
			fmt.Printf("\n- %s: %v", k, v)
		}
		fmt.Printf("\n\n## idents:")
		for k := range identifiers {
			fmt.Printf("\n- %s", k)
		}
	}

	//--------------------------------------------------------
	// 2) Construct a multi-directed graph
	//--------------------------------------------------------
	g, nodeByFile, fileSet := r.buildFileGraph(defines, references, identifiers, mentionedIdents)

	// 4) Personalization
	personal := make(map[int64]float64)
	totalFiles := float64(len(fileSet))
	defaultPersonal := 1.0 / totalFiles

	chatSet := make(map[string]struct{})
	for cf := range mentionedFnames {
		chatSet[cf] = struct{}{}
	}

	for f, node := range nodeByFile {
		if _, inChat := chatSet[f]; inChat {
			personal[node.ID()] = 100.0 / totalFiles
		} else {
			personal[node.ID()] = defaultPersonal
		}
	}

	// 5) Run PageRank (NOTE: gonum.network.PageRank might not natively handle personalization
	// the same way. If you need full personalized PageRank, you might have to modify or implement
	// your own. For now, we do unpersonalized for demonstration.)
	pr := network.PageRank(g, 0.85, 1e-6) // no direct personalization used

	// fmt.Println("PageRank results:")
	// PrintStructOut(pr)

	//--------------------------------------------------------
	// 3) Distribute each file’s rank across its out-edges
	//--------------------------------------------------------
	edgeRanks := distributeRank(pr, defines, references, nodeByFile, mentionedIdents)

	if r.Verbose {
		fmt.Printf("\n\n## Ranked defs:")
		for edge, rank := range edgeRanks {
			fmt.Printf("\n- %v / %s / %s", rank, edge.dst, edge.symbol)
		}
	}

	//--------------------------------------------------------
	// 4) Convert edge-based rank to a sorted list
	//--------------------------------------------------------
	defRankSlice := toDefRankSlice(edgeRanks)

	// 8) Sort by rank, then by fname, then by symbol
	sort.Slice(defRankSlice, func(i, j int) bool {
		if defRankSlice[i].rank != defRankSlice[j].rank {
			return defRankSlice[i].rank > defRankSlice[j].rank
		}
		if defRankSlice[i].fname != defRankSlice[j].fname {
			return defRankSlice[i].fname < defRankSlice[j].fname
		}
		return defRankSlice[i].symbol < defRankSlice[j].symbol
	})

	chatRelFnames := make(map[string]bool)
	// If you had a slice of chatFnames, for example:
	/*
		for _, cf := range chatFnamesSlice {
			rel := r.GetRelFname(cf)
			chatRelFnames[rel] = true
		}
	*/
	if r.Verbose {
		fmt.Printf("\n\n## Ranked defs (SORTED):")
		for _, v := range defRankSlice {
			fmt.Printf("\n- %v / %s / %s", v.rank, v.fname, v.symbol)
		}

		fmt.Printf("\n\n")
	}

	//--------------------------------------------------------
	// 5) Gather final tags, skipping chat files if desired
	//--------------------------------------------------------
	var rankedTags []Tag
	for _, dr := range defRankSlice {
		if chatRelFnames[dr.fname] {
			continue
		}
		k := tagKey{fname: dr.fname, symbol: dr.symbol}
		defs := definitions[k]
		rankedTags = append(rankedTags, defs...)
	}

	// Possibly append files that have no tags, etc.
	return rankedTags
}

// edgeData is a small struct to hold adjacency info for distributing rank
type edgeData struct {
	dstFile string
	symbol  string
	weight  float64
}

type EdgeRank struct {
	dst    string
	symbol string
}

type DefRank struct {
	fname  string
	symbol string
	rank   float64
}

// toDefRankSlice converts the map[EdgeRank]rank into a slice for sorting
func toDefRankSlice(edgeRanks map[EdgeRank]float64) []DefRank {
	defRankSlice := make([]DefRank, 0, len(edgeRanks))
	for k, v := range edgeRanks {
		defRankSlice = append(defRankSlice, DefRank{
			fname:  k.dst,
			symbol: k.symbol,
			rank:   v,
		})
	}
	return defRankSlice
}

// distributeRank inspects each node's PageRank, sums the weights of all its out-edges,
// and then distributes that node's rank proportionally along those edges.
// The result is a mapping (defFile, symbol) -> rank. This parallels:
//
//	for src in G.nodes:
//	    srcRank = ranked[src]
//	    totalWeight = sum of out-edge weights
//	    for edge in out-edges:
//	        portion = srcRank * (edgeWeight / totalWeight)
//	        ranked_definitions[(edge.target, edge.symbol)] += portion
func distributeRank(
	pr map[int64]float64,
	defines map[string]map[string]struct{},
	references map[string][]string,
	nodeByFile map[string]graph.Node,
	mentionedIdents map[string]bool,
) map[EdgeRank]float64 {

	// 6) Distribute rank from each src node across its out edges
	edgeRanks := make(map[EdgeRank]float64)

	for symbol, refMap := range references {
		defFiles := defines[symbol]
		if defFiles == nil {
			continue
		}

		// // ndelorme - ranked tags
		// fmt.Println("\n\nRanked tags:")
		// for _, t := range rankedTags {
		// 	fmt.Printf("- %s / %d / %s\n", t.Kind, t.Line, t.Name)
		// }

		var mul float64
		switch {
		case mentionedIdents[symbol]:
			mul = 10.0
		case strings.HasPrefix(symbol, "_"):
			mul = 0.1
		default:
			mul = 1.0
		}

		for _, refFile := range refMap {
			w := mul * math.Sqrt(float64(len(refMap)))
			sumW := float64(len(defFiles)) * w // If each defFile gets w from refFile

			srcRank := pr[nodeByFile[refFile].ID()]
			if sumW == 0 {
				continue
			}
			for defFile := range defFiles {
				portion := srcRank * (w / sumW)
				edgeRanks[struct {
					dst    string
					symbol string
				}{dst: defFile, symbol: symbol}] += portion
			}
		}
	}

	return edgeRanks
}

// buildFileGraph scans the union of (defines, references) to find all unique filenames
// and create a node for each. The return is a MultiDirectedGraph plus a lookup map to
// find that node by filename.
func (r *RepoMap) buildFileGraph(
	defines map[string]map[string]struct{},
	references map[string][]string,
	identifiers map[string]bool,
	mentionedIdents map[string]bool,
) (
	g *multi.WeightedDirectedGraph,
	nodeByFile map[string]graph.Node,
	fileSet map[string]struct{},
) {
	// 2) Build a multi directed graph
	g = multi.NewWeightedDirectedGraph()

	// Keep track of the node ID for each rel_fname
	nodeByFile = make(map[string]graph.Node)

	// Gather all relevant filenames
	fileSet = make(map[string]struct{})
	for _, defFiles := range defines {
		for f := range defFiles {
			fileSet[f] = struct{}{}
		}
	}
	for _, refMap := range references {
		for _, f := range refMap {
			fileSet[f] = struct{}{}
		}
	}

	// Create node for each file
	for f := range fileSet {
		n := g.NewNode()
		g.AddNode(n)
		nodeByFile[f] = n
	}

	if r.Verbose {
		fmt.Printf("\n\nNumber of nodes (files): %d\n", g.Nodes().Len())
	}

	// 3) For each ident, link referencing file -> defining file with weight
	for ident, _ := range identifiers {
		defFiles := defines[ident]
		if len(defFiles) == 0 {
			continue
		}

		var mul float64
		switch {
		case mentionedIdents[ident]:
			mul = 10.0
		case strings.HasPrefix(ident, "_"):
			mul = 0.1
		default:
			mul = 1.0
		}

		for _, refFile := range references[ident] {
			// log.Trace().Msg(color.YellowString("refFile: %s, numRefs: %d"), refFile, numRefs))
			w := mul * math.Sqrt(float64(len(references[ident])))
			for defFile := range defFiles {
				refNode := nodeByFile[refFile]
				defNode := nodeByFile[defFile]

				// Create a weighted edge
				edge := g.NewWeightedLine(refNode, defNode, w)
				g.SetWeightedLine(edge)
			}
		}
	}

	return g, nodeByFile, fileSet
}

// buildReferenceMaps reads a slice of Tag objects and partitions them into
// (symbol -> set of files that define it) and (symbol -> map[file] countOfRefs).
// It also tracks the actual definition Tag objects for (file,symbol).
func (r *RepoMap) buildReferenceMaps(allTags []Tag) (
	defines map[string]map[string]struct{}, // symbol -> set{relFname}
	references map[string][]string, // symbol -> map[relFname] -> # of references
	definitions map[tagKey][]Tag, // (relFname, symbol) -> slices of definition tags
	identifiers map[string]bool, // set of symbols that have both defines and references
) {
	// 1) Collect references, definitions
	// defines is a set of filenames that define a symbol
	defines = make(map[string]map[string]struct{}) // symbol -> set of filenames that define it
	// references is a list of files per symbol
	references = make(map[string][]string) // symbol -> map of (referencerFile -> countOfRefs)
	// definitions is a set of symbols (tags) including file where they are defined
	definitions = make(map[tagKey][]Tag) // (fname, symbol) -> slice of definition Tags

	for _, t := range allTags {
		rel := r.GetRelFname(t.FilePath)

		switch t.Kind {
		case TagKindDef:
			if defines[t.Name] == nil {
				defines[t.Name] = make(map[string]struct{})
			}
			defines[t.Name][rel] = struct{}{}

			k := tagKey{fname: rel, symbol: t.Name}
			definitions[k] = append(definitions[k], t)

		case TagKindRef:
			// if references[t.Name] == nil {
			// 	references[t.Name] = map[string][]string{t.FileName: {rel}}
			// }
			references[t.Name] = append(references[t.Name], rel)
		}
	}

	// If references is empty, fall back to references=defines
	// this code is needed as page rank will not work if references is empty
	if len(references) == 0 {
		for sym, defFiles := range defines {
			for df := range defFiles {
				references[sym] = append(references[sym], df)
			}
		}
	}

	// idents = set(defines.keys()).intersection(set(references.keys()))
	//
	identifiers = make(map[string]bool)
	for sym := range defines {
		if _, ok := references[sym]; ok {
			identifiers[sym] = true
		}
	}

	return defines, references, definitions, identifiers
}

// fallbackReferences is used when no references are found. Python code sets references = defines,
// effectively giving each symbol a trivial reference from its own definer.
func (r *RepoMap) fallbackReferences(defines map[string]map[string]struct{}) map[string]map[string]int {
	refs := make(map[string]map[string]int)
	for sym, defFiles := range defines {
		refs[sym] = make(map[string]int)
		for df := range defFiles {
			// Just increment by 1 to indicate a trivial reference from the def file to itself
			refs[sym][df]++
		}
	}
	return refs
}

// GetRankedTagsMap orchestrates calls to getRankedTags and toTree to produce the final “map” string.
func (r *RepoMap) GetRankedTagsMap(
	chatFnames, otherFnames []string,
	maxMapTokens int,
	mentionedFnames, mentionedIdents map[string]bool,
	forceRefresh bool,
) string {

	startTime := time.Now()

	// Combine chatFnames and otherFnames into a map of unique elements
	allFnames := uniqueElements(chatFnames, otherFnames)

	// Collect all tags from those files
	allTags := r.getTagsFromFiles(allFnames, commonWords)

	// Handle empty tag list
	if len(allTags) == 0 {
		return ""
	}

	// Get ranked tags by PageRank
	rankedTags := r.getRankedTagsByPageRank(allTags, mentionedFnames, mentionedIdents)

	// special := filterImportantFiles(otherFnames)

	// // Prepend special files as “important”.
	// var specialTags []Tag
	// for _, sf := range special {
	// 	specialTags = append(specialTags, Tag{Name: r.GetRelFname(sf)})
	// }
	// finalTags := append(specialTags, rankedTags...)

	finalTags := rankedTags

	bestTree := ""
	// bestTreeTokens := 0.0

	// lb := 0
	ub := len(finalTags)
	middle := ub
	if middle > 30 {
		middle = 30
	}

	bestTree = r.toTree(finalTags, chatFnames)

	// for lb <= ub {
	// 	tree := r.toTree(finalTags[:middle], chatFnames)
	// 	numTokens := r.TokenCount(tree)

	// 	diff := math.Abs(numTokens - float64(maxMapTokens))
	// 	pctErr := diff / float64(maxMapTokens)
	// 	if (numTokens <= float64(maxMapTokens) && numTokens > bestTreeTokens) || pctErr < 0.15 {
	// 		bestTree = tree
	// 		bestTreeTokens = numTokens
	// 		if pctErr < 0.15 {
	// 			break
	// 		}
	// 	}
	// 	if numTokens < float64(maxMapTokens) {
	// 		lb = middle + 1
	// 	} else {
	// 		ub = middle - 1
	// 	}
	// 	middle = (lb + ub) / 2
	// }

	endTime := time.Now()
	r.MapProcessingTime = endTime.Sub(startTime).Seconds()

	r.LastMap = bestTree
	return bestTree
}

// tr@ck -- improve this chat vs other files. We should have repoFiles and chatFiles

type RepoMapOptions struct {
	Color                    bool
	Verbose                  bool
	ShowLineNumber           bool
	ShowParentContext        bool
	ShowChildContext         bool
	ShowLastLine             bool
	MarginPadding            int
	MarkLinesOfInterest      bool
	HeaderMax                int
	ShowTopOfFileParentScope bool
	LinesOfInterestPadding   int
}

// GenerateRepoMap is the top-level function (mirroring the Python method) that produces the “repo content”.
func (r *RepoMap) GenerateRepoMap(
	chatFiles, otherFiles []string,
	mentionedFnames, mentionedIdents map[string]bool,
	forceRefresh bool,
) string {

	if r.MaxMapTokens <= 0 {
		log.Warn().Msgf("Repo-map disabled by max_map_tokens: %d", r.MaxMapTokens)
		return ""
	}
	// if len(otherFiles) == 0 {
	// 	log.Warn().Msg("No other files found; disabling repo map")
	// 	return ""
	// }
	if mentionedFnames == nil {
		mentionedFnames = make(map[string]bool)
	}
	if mentionedIdents == nil {
		mentionedIdents = make(map[string]bool)
	}

	maxMapTokens := r.MaxMapTokens
	padding := 4096
	var target int
	if maxMapTokens > 0 && r.MaxCtxWindow > 0 {
		t := maxMapTokens * r.MapMulNoFiles
		t2 := r.MaxCtxWindow - padding
		if t2 < 0 {
			t2 = 0
		}
		if t < t2 {
			target = t
		} else {
			target = t2
		}
	}
	if len(chatFiles) == 0 && r.MaxCtxWindow > 0 && target > 0 {
		maxMapTokens = target
	}

	var filesListing string
	// defer func() {
	// 	if rec := recover(); rec != nil {
	// 		fmt.Printf("ERR: Disabling repo map, repository may be too large?")
	// 		r.MaxMapTokens = 0
	// 		filesListing = ""
	// 	}
	// }()

	filesListing = r.GetRankedTagsMap(chatFiles, otherFiles, maxMapTokens, mentionedFnames, mentionedIdents, forceRefresh)
	if filesListing == "" {
		return ""
	}

	if r.Verbose {
		numTokens := r.TokenCount(filesListing)
		fmt.Printf("Repo-map: %.1f k-tokens\n", numTokens/1024.0)
	}

	other := ""
	if len(chatFiles) > 0 {
		other = "other "
	}

	var repoContent string
	if r.RepoContentPx != "" {
		repoContent = strings.ReplaceAll(r.RepoContentPx, "{other}", other)
	}

	repoContent += filesListing
	return repoContent
}

// toTree converts a list of Tag objects into a tree-like string representation.
func (r *RepoMap) toTree(tags []Tag, chatFnames []string) string {
	// Return immediately if no tags
	if len(tags) == 0 {
		return ""
	}

	// 1) Build a set of relative filenames that should be skipped
	chatRelSet := make(map[string]bool)
	for _, c := range chatFnames {
		rel := r.GetRelFname(c)
		chatRelSet[rel] = true
	}

	// tr@ck - verbose
	for i, c := range chatFnames {
		log.Debug().Int("index", i).Str("file", c).Msg("chat files")
	}

	//  2) Sort the tags first by FileName in ascending order, and then by Line in ascending order
	// if two tags have the same FileName. This ensures a stable order where entries
	// are grouped by file and appear sequentially by their line numbers within each file.
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].FileName != tags[j].FileName {
			return tags[i].FileName < tags[j].FileName
		}
		return tags[i].Line < tags[j].Line
	})

	// A sentinel value used to trigger a final flush of the current file's data in a streaming process.
	sentinel := "__sentinel_tag__"

	// 3) Append a sentinel tag, which triggers the final flush when we hit it in the loop.
	tags = append(tags, Tag{FileName: sentinel, Name: sentinel})

	// 4) Prepare to walk through each tag, grouping them by file.
	var output strings.Builder

	var curFname string    // Tracks the *relative* file name of the current group
	var curAbsFname string // Tracks the absolute path for rendering
	var linesOfInterest []int

	// sort tags by line number

	// 5) Process tags in a streaming fashion, flushing out each file's lines-of-interest
	//    when we detect a "new file name" or the dummy tag.
	for i, t := range tags {
		log.Debug().Int("index", i).Str("file", t.FileName).Int("line", t.Line).Str("tag", t.Name).Msg("tags")

		relFname := t.FileName
		// // Skip tags that belong to a “chat” file. (Python: if this_rel_fname in chat_rel_fnames: continue)
		// if chatRelSet[relFname] {
		// 	continue
		// }

		// If we've encountered a new file (i.e., the file name changed),
		// flush out the old file's lines-of-interest (if any).
		if relFname != curFname {
			if curFname != "" && linesOfInterest != nil {
				// Write a blank line, then the file name plus colon
				output.WriteString("\n" + curFname + ":\n")

				code, err := os.ReadFile(curAbsFname)
				if err != nil {
					log.Warn().Err(err).Msgf("Failed to read file (%s)", curAbsFname)
					continue
				}

				// Render the code snippet for the previous file.
				rendered, err := r.renderTree(curFname, code, linesOfInterest)
				if err != nil {
					// If there's an error reading or parsing the file, just log and move on.
					log.Warn().Err(err).Msgf("Failed to render tree for %s", curFname)
				}
				output.WriteString(rendered)
			}

			// If the new file name is the dummy sentinel, we've reached the end; stop.
			if relFname == sentinel {
				break
			}

			// Otherwise, reset our state for the *new* file.
			curFname = relFname
			curAbsFname = t.FilePath
			linesOfInterest = []int{}
		}

		// Accumulate the line number from this tag for the current file.
		if linesOfInterest != nil {
			linesOfInterest = append(linesOfInterest, t.Line)
		}
	}

	// 6) Truncate lines in the final output, in case of minified or extremely long content.
	//    This matches the Python code that does:  line[:100] for line in output.splitlines()
	lines := strings.Split(output.String(), "\n")
	for i, ln := range lines {
		if len(ln) > 100 {
			lines[i] = ln[:100]
		}
	}

	// 7) Return the final output (plus a newline).
	return strings.Join(lines, "\n") + "\n"
}

// renderTree uses a grep-ast TreeContext to produce a nice snippet with lines of interest expanded.
func (r *RepoMap) renderTree(relFname string, code []byte, linesOfInterest []int) (string, error) {
	if r.Verbose {
		fmt.Printf("\nrender_tree:  %s, %v\n", relFname, linesOfInterest)
	}

	// Build a grep-ast TreeContext.
	// (Below is an example usage; adapt to whatever the actual library API provides.)
	tc, err := grepast.NewTreeContext(
		relFname, code,
		grepast.WithColor(false), // todo
		grepast.WithChildContext(false),
		grepast.WithLastLineContext(false),
		grepast.WithTopMargin(0),
		grepast.WithLinesOfInterestMarked(false),
		grepast.WithLinesOfInterestPadding(2),
		grepast.WithTopOfFileParentScope(false),
	)
	if err != nil {
		if err == grepast.ErrorUnsupportedLanguage || err == grepast.ErrorUnrecognizedFiletype {
			return "", nil
		}
		return "", fmt.Errorf("failed to create tree context: %w", err)
	}

	// Convert []int to map[int]struct{}
	// tr@ck -- could this be avoided?
	loiMap := make(map[int]struct{}, len(linesOfInterest))
	for _, ln := range linesOfInterest {
		loiMap[ln] = struct{}{}
	}

	// fmt.Println(loiMap)
	// Add the lines of interest
	tc.AddLinesOfInterest(loiMap)
	// Expand context around those lines
	tc.AddContext()

	res := tc.Format()

	return res, nil
}

// getRandomColor replicates the Python get_random_color using HSV → RGB.
func getRandomColor() string {
	hue := rand.Float64()
	r, g, b := hsvToRGB(hue, 1.0, 0.75)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// hsvToRGB is standard. h, s, v in [0,1], output in [0,255].
func hsvToRGB(h, s, v float64) (int, int, int) {
	var r, g, b float64
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)

	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	case 5:
		r, g, b = v, p, q
	}
	return int(r * 255), int(g * 255), int(b * 255)
}

// GetRepoFiles gathers all files in a directory (or the file itself).
func GetRepoFiles(path string, gi *goignore.GitIgnore) []string {
	info, err := os.Stat(path)
	if err != nil {
		return []string{}
	}

	if !info.IsDir() {
		return []string{path}
	}

	var srcFiles []string
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		// Skip files that match the ignore patterns
		if err != nil {
			return nil
		}
		// Skip files that match the ignore patterns
		if gi.MatchesPath(p) {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		log.Debug().Str("op", "source files").Str("path", p).Msg("add")
		srcFiles = append(srcFiles, p)
		return nil
	})
	return srcFiles
}
