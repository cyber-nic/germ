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

func uniqueElements(slices ...[]string) []string {
	uniqueMap := make(map[string]struct{})
	result := []string{}

	for _, slice := range slices {
		for _, elem := range slice {
			if _, exists := uniqueMap[elem]; !exists {
				uniqueMap[elem] = struct{}{}
				result = append(result, elem)
			}
		}
	}

	return result
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

var commonWords = map[string]struct{}{
	// Common English words
	"the": {}, "and": {}, "for": {}, "with": {}, "this": {}, "from": {}, "into": {},
	"all": {}, "has": {}, "not": {}, "its": {}, "per": {}, "new": {}, "many": {},

	// Go keywords and common types
	"var": {}, "func": {}, "type": {}, "struct": {}, "interface": {}, "msgf": {},
	"string": {}, "strings": {}, "bool": {}, "byte": {}, "error": {}, "uint": {}, "warn": {},
	"range": {}, "return": {}, "case": {}, "map": {}, "make": {}, "sprintf": {},
	"append": {}, "len": {}, "print": {}, "println": {}, "float32": {},
	"float64": {}, "int64": {}, "int32": {}, "int16": {}, "int8": {}, "uint64": {},
	"uint32": {}, "uint16": {}, "uint8": {}, "uintptr": {}, "complex64": {},
	"complex128": {}, "chan": {}, "go": {}, "select": {}, "defer": {}, "panic": {},

	// Python keywords and builtins
	"def": {}, "class": {}, "self": {}, "none": {}, "true": {}, "false": {},
	"dict": {}, "tuple": {}, "int": {}, "str": {}, "float": {}, "import": {},
	"except": {}, "raise": {}, "finally": {},

	// JavaScript/TypeScript keywords and types
	"let": {}, "const": {}, "function": {}, "undefined": {}, "never": {},
	"object": {}, "promise": {}, "number": {}, "boolean": {}, "any": {},
	"prototype": {}, "constructor": {}, "extends": {}, "implements": {},

	// Ruby keywords and common terms
	"module": {}, "require": {}, "attr": {}, "puts": {},
	"ruby": {}, "gem": {}, "rake": {}, "proc": {}, "hash": {}, "symbol": {},

	// Java keywords and common terms
	"public": {}, "private": {}, "protected": {}, "static": {}, "final": {},
	"integer": {}, "exception": {}, "override": {}, "super": {}, "package": {},

	// C# keywords and common terms
	"namespace": {}, "using": {}, "sealed": {}, "virtual": {}, "enum": {},
	"delegate": {}, "event": {}, "task": {}, "dynamic": {}, "linq": {},

	// C++ keywords and common terms
	"template": {}, "typename": {}, "inline": {}, "explicit": {},
	"operator": {}, "friend": {}, "typedef": {}, "sizeof": {}, "auto": {},

	// Common variable names and suffixes
	"err": {}, "src": {}, "dst": {}, "tmp": {}, "ptr": {}, "size": {},
	"impl": {}, "ctx": {}, "msg": {}, "dir": {}, "fmt": {}, "count": {},
	"obj": {}, "arr": {}, "num": {}, "buf": {}, "idx": {}, "pos": {},

	// Common OOP terms
	"base": {}, "derived": {}, "concrete": {}, "factory": {},
	"singleton": {}, "builder": {}, "adapter": {}, "proxy": {}, "facade": {},
	"model": {}, "view": {}, "controller": {}, "service": {}, "repository": {},
	"manager": {}, "handler": {}, "wrapper": {}, "decorator": {}, "observer": {},

	// Common testing terms
	"test": {}, "mock": {}, "stub": {}, "assert": {}, "expect": {},
	"setup": {}, "teardown": {}, "suite": {}, "spec": {}, "benchmark": {},

	// Common programming terms
	"async": {}, "await": {}, "lambda": {}, "yield": {}, "nil": {}, "log": {}, "exit": {},
	"null": {}, "array": {}, "list": {}, "void": {}, "tree": {}, "key": {}, "keys": {},
	"init": {}, "get": {}, "set": {}, "read": {}, "write": {}, "api": {}, "url": {},
	"open": {}, "close": {}, "start": {}, "end": {}, "process": {}, "fatal": {}, "time": {},
	"handle": {}, "create": {}, "delete": {}, "update": {}, "find": {}, "search": {},
	"check": {}, "parse": {}, "convert": {}, "split": {}, "join": {}, "uri": {}, "errorf": {},
	"ignore": {}, "skip": {}, "valid": {}, "match": {}, "text": {}, "line": {}, "printf": {},
	"value": {}, "values": {}, "current": {}, "content": {}, "source": {}, "call": {},
	"child": {}, "children": {}, "parent": {}, "root": {}, "leaf": {}, "each": {},
	"path": {}, "file": {}, "files": {}, "name": {}, "names": {}, "item": {}, "regex": {},
	"code": {}, "data": {}, "input": {}, "output": {}, "debug": {}, "add": {}, "wait": {},
	"abstract": {}, "slice": {}, "node": {}, "request": {}, "response": {}, "info": {}, "trim": {},
	"next": {}, "prev": {}, "first": {}, "last": {}, "min": {}, "max": {}, "sum": {}, "avg": {},
	"copy": {}, "move": {}, "swap": {}, "sort": {}, "filter": {}, "replace": {},
	"include": {}, "exclude": {}, "merge": {}, "diff": {}, "patch": {}, "apply": {},
	"trace": {},
}
