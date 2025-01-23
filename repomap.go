package orb

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	// Import the grep-ast library
	grepast "github.com/cyber-nic/grep-ast"
	"github.com/fatih/color"
	"github.com/rs/zerolog/log"
	sitter "github.com/tree-sitter/go-tree-sitter"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
)

// ------------------------------------------------------------------------------------
// Structs and Stubs
// ------------------------------------------------------------------------------------

const (
	TagKindDef = "def"
	TagKindRef = "ref"
)

// Tag represents a “tag” extracted from a source file.
type Tag struct {
	Name string
	Path string
	Line int
	Text string
	Kind string
}

var (
	ErrOperational = errors.New("operational error")
	ErrDatabase    = errors.New("database error")
)

// TreeContextCacheItem holds the (rendered) context and file mtime for caching.
type TreeContextCacheItem struct {
	Context string
	Mtime   int64
}

// RepoMap is the Go equivalent of the Python class `RepoMap`.
type RepoMap struct {
	CacheVersion      int
	TagsCacheDir      string
	WarnedFiles       map[string]bool
	Refresh           string
	Verbose           bool
	Root              string
	MainModel         *ModelStub
	RepoContentPx     string
	MaxMapTokens      int
	MaxCtxWindow      int
	MapMulNoFiles     int
	TreeCache         map[string]string // For storing rendered code trees
	TreeContextCache  map[string]TreeContextCacheItem
	MapCache          map[string]string
	MapProcessingTime float64
	LastMap           string
	TagsCache         CacheStub
}

// ModelStub simulates the main_model used in Python code (for token_count, etc.).
type ModelStub struct{}

// CacheStub simulates the diskcache.Cache used in Python (here, just an in-memory map).
type CacheStub struct {
	mu    sync.RWMutex
	store map[string]interface{}
}

// ------------------------------------------------------------------------------------
// Implementations
// ------------------------------------------------------------------------------------

// NewCacheStub initializes an in-memory “cache”.
func NewCacheStub() *CacheStub {
	return &CacheStub{
		store: make(map[string]interface{}),
	}
}

func (c *CacheStub) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.store[key]
	return val, ok
}

func (c *CacheStub) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

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
) *RepoMap {
	if root == "" {
		cwd, err := os.Getwd()
		if err == nil {
			root = cwd
		}
	}

	r := &RepoMap{
		CacheVersion:     3,
		TagsCacheDir:     fmt.Sprintf(".aider.tags.cache.v%d", 3),
		WarnedFiles:      make(map[string]bool),
		Refresh:          refresh,
		Verbose:          verbose,
		Root:             root,
		MainModel:        mainModel,
		RepoContentPx:    repoContentPrefix,
		MaxMapTokens:     maxMapTokens,
		MapMulNoFiles:    mapMulNoFiles,
		MaxCtxWindow:     maxContextWindow,
		TreeCache:        make(map[string]string),
		TreeContextCache: make(map[string]TreeContextCacheItem),
		MapCache:         make(map[string]string),
		TagsCache:        *NewCacheStub(),
	}

	if verbose {
		fmt.Printf("RepoMap initialized with map_mul_no_files: %d\n", mapMulNoFiles)
	}
	return r
}

// LoadTagsCache is a stub. In Python, we'd open an on-disk cache here.
func (r *RepoMap) LoadTagsCache() {}

// SaveTagsCache is a stub for persisting changes to disk in Python.
func (r *RepoMap) SaveTagsCache() {}

// ------------------------------------------------------------------------------------
// Helper Methods
// ------------------------------------------------------------------------------------

// GetMtime returns the file's modification time in Unix seconds, or -1 on error.
func (r *RepoMap) GetMtime(fname string) int64 {
	fi, err := os.Stat(fname)
	if err != nil {
		log.Warn().Err(err).Msgf("File not found or stat error: %s", fname)
		return -1
	}
	return fi.ModTime().Unix()
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

// ------------------------------------------------------------------------------------
// Tag Extraction: getTags, getTagsRaw
// ------------------------------------------------------------------------------------

// GetTags checks the in-memory cache first, and if not found (or outdated), calls GetTagsRaw.
func (r *RepoMap) GetTags(fname, relFname string) ([]Tag, error) {
	fileMtime := r.GetMtime(fname)
	if fileMtime < 0 {
		return nil, fmt.Errorf("failed to get mtime for file: %s", fname)
	}

	// Check the cache
	cacheKey := fname
	val, ok := r.TagsCache.Get(cacheKey)
	if ok {
		stored, _ := val.(map[string]interface{})
		if stored != nil {
			mt, _ := stored["mtime"].(int64)
			if mt == fileMtime {
				data, _ := stored["data"].([]Tag)
				return data, nil
			}
		}
	}
	// Not cached or changed; re-parse
	data, err := r.GetTagsRaw(fname, relFname)
	if err != nil {
		return nil, err
	}

	if data == nil {
		data = nil
	}

	newStore := map[string]interface{}{
		"mtime": fileMtime,
		"data":  data,
	}
	r.TagsCache.Set(cacheKey, newStore)
	r.SaveTagsCache()
	return data, nil
}

func getSourceCodeMapQuery(lang string) ([]byte, error) {
	tpl := "queries/tree-sitter-%s-tags.scm"

	switch lang {
	case "go", "python", "javascript", "typescript", "ruby", "java", "c", "cpp":
		queryFilename := fmt.Sprintf(tpl, lang)
		// log.Trace().Str("lang", lang).Str("file", queryFilename).Msg("getSourceCodeMapQuery")

		return os.ReadFile(queryFilename)
	default:
		return []byte{}, fmt.Errorf("unsupported language: %s", lang)
	}
}

var errUnsupportedFileType = errors.New("unsupported file type")

// GetTagsRaw parses the file with Tree-sitter and extracts "function definitions"
func (r *RepoMap) GetTagsRaw(fname, relFname string) ([]Tag, error) {
	// 1) Identify the file's language
	lang, l, err := grepast.GetLanguageFromFileName(fname)
	if err != nil || lang == nil {
		return nil, errUnsupportedFileType
	}

	// 3) Create parser
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	// 2) Read source code
	sourceCode, err := os.ReadFile(fname)
	if err != nil {
		return nil, fmt.Errorf("failed to read file (%s): %v", fname, err)
	}
	if len(sourceCode) == 0 {
		return nil, fmt.Errorf("empty file: %s", fname)
	}

	// 4) Parse
	tree := parser.Parse(sourceCode, nil)
	if tree == nil || tree.RootNode() == nil {
		return nil, fmt.Errorf("failed to parse file: %s", fname)
	}

	log.Debug().Str("lang", l).Str("file", fname).Msg("sitter")

	// 5) Load your query
	querySource, err := getSourceCodeMapQuery(l)
	if err != nil {
		return nil, fmt.Errorf("failed to read query file (%s): %v", l, err)
	}
	if len(querySource) == 0 {
		return nil, fmt.Errorf("empty query file: %s", l)
	}

	q, qErr := sitter.NewQuery(lang, string(querySource))
	if qErr != nil {
		var queryErr *sitter.QueryError
		if errors.As(qErr, &queryErr) {
			if queryErr != nil {
				return nil, fmt.Errorf("query error: %s at row: %d, column: %d, offset: %d, kind: %v",
					queryErr.Message, queryErr.Row, queryErr.Column, queryErr.Offset, queryErr.Kind)
			}
			return nil, fmt.Errorf("unexpected nil *sitter.QueryError")
		}
		return nil, fmt.Errorf("failed to create query: %v", qErr)
	}
	defer q.Close()

	// 6) Execute the query
	qc := sitter.NewQueryCursor()
	defer qc.Close()

	// Iterate over all of the individual captures in the order that they appear.
	captures := qc.Captures(q, tree.RootNode(), sourceCode)

	var tags []Tag
	for match, index := captures.Next(); match != nil; match, index = captures.Next() {
		c := match.Captures[index]

		if c.Node.ChildCount() == 0 {
			continue
		}

		// Get the name of the capture
		captureName := q.CaptureNames()[index]
		// log.Debug().Str("captureName", captureName).Uint("sitter", index).Msg("sitter")

		// Decide if it's a definition or reference
		switch {
		case strings.HasPrefix(captureName, "name.definition."):
			// E.g. "name.definition.function", "name.definition.method", etc.
			row := c.Node.StartPosition().Row
			t := string(c.Node.Utf8Text(sourceCode)) // or node.Utf8Text(code)

			tags = append(tags, Tag{
				Name: relFname,
				Path: fname,
				Line: int(row),
				Text: t,
				Kind: TagKindDef,
			})

		case strings.HasPrefix(captureName, "name.reference."):
			// E.g. "name.reference.call", "name.reference.type", ...
			row := c.Node.StartPosition().Row
			name := string(c.Node.Utf8Text(sourceCode))

			tags = append(tags, Tag{
				Name: relFname,
				Path: fname,
				Line: int(row),
				Text: name,
				Kind: TagKindRef,
			})

		default:
			// Not a captured name we care about
		}

	}

	// 7) Return the list of Tag objects
	return tags, nil
}

// ------------------------------------------------------------------------------------
// Ranked Tags + RepoMap generation
// ------------------------------------------------------------------------------------

// getTagsFromFiles Collect all tags from those files
func (r *RepoMap) getTagsFromFiles(
	allFnames map[string]struct{},
	progress func(),
) []Tag {

	var allTags []Tag

	for fname, _ := range allFnames {
		if progress != nil {
			progress()
		}
		fi, err := os.Stat(fname)
		if err != nil || fi.IsDir() {
			if !r.WarnedFiles[fname] {
				log.Warn().Err(err).Msgf("Repo-map can't include %s", fname)
				fmt.Println("Has it been deleted from the file system but not from git?")
				r.WarnedFiles[fname] = true
			}
			continue
		}

		rel := r.GetRelFname(fname)
		tg, err := r.GetTags(fname, rel)
		if err != nil {
			if err == errUnsupportedFileType {
				log.Trace().Msgf("skip %s", fname)
			} else {
				log.Warn().Err(err).Msgf("Failed to get tags for %s", fname)
			}
			continue
		}
		if tg != nil {
			allTags = append(allTags, tg...)
		}
	}
	return allTags
}

// getRankedTagsSimple is a simple ranking algorithm based on the number of defs per file.
func (r *RepoMap) getRankedTagsSimple(
	allTags []Tag,
	mentionedFnames, mentionedIdents map[string]bool,
	progress func(),
) []Tag {
	log.Trace().Msg(color.YellowString("getRankedTagsSimple"))

	// tr@ck
	// Instead of a true pagerank, we do a naive sorting by # of defs per file.
	defScore := make(map[string]int)
	for _, t := range allTags {
		if t.Kind == TagKindDef {
			defScore[t.Name]++
		}
	}

	PrintStructOut(defScore)

	// Sort allTags by defScore descending, then by filename, line ascending.
	sortable := make([]Tag, len(allTags))
	copy(sortable, allTags)

	simpleSort(sortable, func(a, b Tag) bool {
		as := defScore[a.Name]
		bs := defScore[b.Name]
		if as != bs {
			return as > bs
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Line < b.Line
	})

	return sortable
}

// getRankedTagsByPageRank is a more sophisticated ranking algorithm based on PageRank.
func (r *RepoMap) getRankedTagsByPageRank(
	allTags []Tag,
	mentionedFnames, mentionedIdents map[string]bool,
	progress func(),
) []Tag {
	log.Trace().Msg(color.YellowString("getRankedTagsByPageRank (PageRank)"))

	// Build a directed graph where each Tag is a node
	dg := simple.NewDirectedGraph()

	// nodeFor[i] will be the gonum Node for allTags[i]
	nodeFor := make([]graph.Node, len(allTags))

	// We'll also keep an index->ID map so we can retrieve IDs if needed
	for i := range allTags {
		n := dg.NewNode()
		dg.AddNode(n)
		nodeFor[i] = n
	}

	// Group tags by identical Name
	byName := make(map[string][]int)
	for i, t := range allTags {
		byName[t.Name] = append(byName[t.Name], i)
	}

	// For each name, link references → definitions with directed edges
	for _, indices := range byName {
		var refIndices, defIndices []int
		for _, idx := range indices {
			if allTags[idx].Kind == TagKindDef {
				defIndices = append(defIndices, idx)
			} else if allTags[idx].Kind == TagKindRef {
				refIndices = append(refIndices, idx)
			}
		}
		if len(refIndices) == 0 || len(defIndices) == 0 {
			continue
		}
		// Create edges from each ref to each def
		for _, rIdx := range refIndices {
			for _, dIdx := range defIndices {
				// Currently, edges are added without specifying weights (NewEdge instead of NewWeightedEdge).
				// This treats all edges equally. That might be correct for your domain, but if some edges (references) are more “important” than others,
				// a weighted approach could yield different results.
				dg.SetEdge(dg.NewEdge(nodeFor[rIdx], nodeFor[dIdx]))
			}
		}
	}

	// Run PageRank (unweighted) on the constructed graph

	// 0.85 is famously the default used in the original PageRank paper. It’s a very common choice and generally a safe, “industry-standard” default
	damp := 0.85
	// 0.000001 is also a typical. A tolerance of 1e-6 strikes a reasonable balance between accuracy and performance for most moderately sized graphs
	tol := 0.000001
	pr := network.PageRank(dg, damp, tol)

	// Convert the page-rank mapping from node ID -> rank into “index -> rank”
	// since each node corresponds to allTags[i].
	scores := make([]float64, len(allTags))
	for i, node := range nodeFor {
		scores[i] = pr[node.ID()]
	}

	// Sort allTags by PageRank descending, then by name, then by line
	sortable := make([]int, len(allTags))
	for i := range sortable {
		sortable[i] = i
	}

	sort.Slice(sortable, func(a, b int) bool {
		iA, iB := sortable[a], sortable[b]
		sA, sB := scores[iA], scores[iB]
		if sA != sB {
			return sA > sB // higher rank first
		}
		nameA, nameB := allTags[iA].Name, allTags[iB].Name
		if nameA != nameB {
			return nameA < nameB
		}
		return allTags[iA].Line < allTags[iB].Line
	})

	// Rebuild sorted Tag slice
	ranked := make([]Tag, len(allTags))
	for i, idx := range sortable {
		ranked[i] = allTags[idx]
	}

	return ranked
}

// GetRankedTagsMap orchestrates calls to getRankedTags and toTree to produce the final “map” string.
func (r *RepoMap) GetRankedTagsMap(
	chatFnames, otherFnames []string,
	maxMapTokens int,
	mentionedFnames, mentionedIdents map[string]bool,
	forceRefresh bool,
) string {
	log.Trace().Msg(color.YellowString("GetRankedTagsMap"))

	cacheKey := strings.Join(chatFnames, "|") + "||" + strings.Join(otherFnames, "|") +
		fmt.Sprintf("||%d", maxMapTokens)

	if r.Refresh != "always" && !forceRefresh {
		if val, ok := r.MapCache[cacheKey]; ok {
			return val
		}
	}

	startTime := time.Now()

	// Combine chatFnames and otherFnames into a map of unique elements
	allFnames := uniqueElements(chatFnames, otherFnames)

	// Collect all tags from those files
	allTags := r.getTagsFromFiles(allFnames, nil)

	rankedTags := r.getRankedTagsSimple(allTags, mentionedFnames, mentionedIdents, nil)

	// rankedTags := r.getRankedTagsByPageRank(allTags, mentionedFnames, mentionedIdents, nil)
	// tr@ck
	for i, t := range rankedTags {
		if t.Name == "doc" {
			continue
		}
		// first 10 chars of the text
		log.Trace().Int("index", i).Str("path", t.Path).Int("line", t.Line).Str("text", fmt.Sprintf("%s ...", t.Text[:20])).Str("kind", t.Kind).Msg("ranked tags")
	}

	special := filterImportantFiles(otherFnames)

	// Prepend special files as “important”.
	var specialTags []Tag
	for _, sf := range special {
		specialTags = append(specialTags, Tag{Name: r.GetRelFname(sf)})
	}
	finalTags := append(specialTags, rankedTags...)

	bestTree := ""
	bestTreeTokens := 0.0

	lb := 0
	ub := len(finalTags)
	middle := ub
	if middle > 30 {
		middle = 30
	}

	for lb <= ub {
		tree := r.toTree(finalTags[:middle], chatFnames)
		numTokens := r.TokenCount(tree)

		diff := math.Abs(numTokens - float64(maxMapTokens))
		pctErr := diff / float64(maxMapTokens)
		if (numTokens <= float64(maxMapTokens) && numTokens > bestTreeTokens) || pctErr < 0.15 {
			bestTree = tree
			bestTreeTokens = numTokens
			if pctErr < 0.15 {
				break
			}
		}
		if numTokens < float64(maxMapTokens) {
			lb = middle + 1
		} else {
			ub = middle - 1
		}
		middle = (lb + ub) / 2
	}

	endTime := time.Now()
	r.MapProcessingTime = endTime.Sub(startTime).Seconds()

	r.MapCache[cacheKey] = bestTree
	r.LastMap = bestTree
	return bestTree
}

// GetRepoMap is the top-level function (mirroring the Python method) that produces the “repo content”.
func (r *RepoMap) GetRepoMap(
	chatFiles, otherFiles []string,
	mentionedFnames, mentionedIdents map[string]bool,
	forceRefresh bool,
) string {
	log.Trace().Msg(color.YellowString("GetRepoMap"))

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
	defer func() {
		if rec := recover(); rec != nil {
			fmt.Printf("ERR: Disabling repo map, repository may be too large?")
			r.MaxMapTokens = 0
			filesListing = ""
		}
	}()

	filesListing = r.GetRankedTagsMap(chatFiles, otherFiles, maxMapTokens, mentionedFnames, mentionedIdents, forceRefresh)
	if filesListing == "" {
		return ""
	}

	fmt.Println(filesListing)

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

// ------------------------------------------------------------------------------------
// Rendering code blocks with TreeContext from grep-ast
// ------------------------------------------------------------------------------------
func (r *RepoMap) toTree(tags []Tag, chatFnames []string) string {
	if len(tags) == 0 {
		return ""
	}
	chatRelSet := make(map[string]bool)
	for _, c := range chatFnames {
		chatRelSet[r.GetRelFname(c)] = true
	}

	curFname := ""
	curAbsFname := ""
	var linesOfInterest []int
	var output strings.Builder

	dummyTag := Tag{Name: "____dummy____"}
	tagsWithDummy := append(tags, dummyTag)

	for _, tag := range tagsWithDummy {
		if chatRelSet[tag.Name] {
			continue
		}
		if tag.Name != curFname {
			if curFname != "" && linesOfInterest != nil {
				output.WriteString("\n" + curFname + ":\n")
				rendered := r.renderTree(curAbsFname, curFname, linesOfInterest)
				output.WriteString(rendered)
			}
			if tag.Name == "____dummy____" {
				break
			}
			linesOfInterest = []int{}
			curFname = tag.Name
			curAbsFname = tag.Path
		}
		if linesOfInterest != nil {
			linesOfInterest = append(linesOfInterest, tag.Line)
		}
	}

	lines := strings.Split(output.String(), "\n")
	for i, ln := range lines {
		if len(ln) > 100 {
			lines[i] = ln[:100]
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// renderTree uses a grep-ast TreeContext to produce a nice snippet with lines of interest expanded.
func (r *RepoMap) renderTree(absFname, relFname string, linesOfInterest []int) string {
	mtime := r.GetMtime(absFname)
	if mtime < 0 {
		return ""
	}

	// Key is (relFname + lines + mtime)
	intsStr := make([]string, 0, len(linesOfInterest))
	for _, v := range linesOfInterest {
		intsStr = append(intsStr, fmt.Sprintf("%d", v))
	}
	cacheKey := relFname + "|" + strings.Join(intsStr, ",") + fmt.Sprintf("|%d", mtime)

	if cached, ok := r.TreeCache[cacheKey]; ok {
		return cached
	}

	code, err := os.ReadFile(absFname)
	if err != nil {
		return ""
	}

	// Build a grep-ast TreeContext.
	// (Below is an example usage; adapt to whatever the actual library API provides.)
	tc, err := grepast.NewTreeContext(relFname, code, grepast.TreeContextOptions{
		Color:                    false,
		Verbose:                  false,
		ShowLineNumber:           false,
		ShowParentContext:        false,
		ShowChildContext:         false,
		ShowLastLine:             false,
		MarginPadding:            0,
		MarkLinesOfInterest:      false,
		HeaderMax:                0,
		ShowTopOfFileParentScope: false,
		LinesOfInterestPadding:   0,
	})

	// Convert []int to map[int]struct{}
	// tr@ck -- could this be avoided?
	loiMap := make(map[int]struct{}, len(linesOfInterest))
	for _, ln := range linesOfInterest {
		loiMap[ln] = struct{}{}
	}

	// Add the lines of interest
	tc.AddLinesOfInterest(loiMap)
	// Expand context around those lines
	tc.AddContext()

	res := tc.Format()

	r.TreeCache[cacheKey] = res
	return res
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

// FindSrcFiles gathers all files in a directory (or the file itself).
func FindSrcFiles(path string) []string {
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
		if grepast.MatchIgnorePattern(p, grepast.DefaultIgnorePatterns) {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		srcFiles = append(srcFiles, p)
		return nil
	})
	return srcFiles
}

// getSupportedLanguagesMD is a stub. The Python version shows a Markdown table of supported languages.
func getSupportedLanguagesMD() string {
	return `
| Language | File extension | Repo map | Linter |
|:--------:|:--------------:|:--------:|:------:|
| Python   | .py           |   ✓      |   ✓    |
| Go       | .go           |   ✓      |   ✓    |
`
}

// ------------------------------------------------------------------------------------
// main function (equivalent to `if __name__ == "__main__":` in Python)
// ------------------------------------------------------------------------------------
func main() {
	args := os.Args
	if len(args) < 2 {
		fmt.Println("Usage: go run repomap.go [files_or_directories...]")
		return
	}
	fnames := args[1:]

	var chatFnames, otherFnames []string
	for _, fname := range fnames {
		fi, err := os.Stat(fname)
		if err == nil && fi.IsDir() {
			chatFnames = append(chatFnames, FindSrcFiles(fname)...)
		} else {
			chatFnames = append(chatFnames, fname)
		}
	}

	rm := NewRepoMap(
		1024,         // mapTokens
		".",          // root
		&ModelStub{}, // mainModel
		"",           // repoContentPrefix
		false,        // verbose
		16000,        // maxContextWindow
		8,            // mapMulNoFiles
		"auto",       // refresh
	)

	// Generate the repo map (a textual summary)
	repoMap := rm.GetRankedTagsMap(chatFnames, otherFnames, 1024, nil, nil, false)
	fmt.Println(len(repoMap))
	fmt.Println(repoMap)
}
