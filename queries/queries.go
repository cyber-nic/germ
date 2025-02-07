package scm

import (
	_ "embed"
	"fmt"
)

//go:embed tree-sitter-c_sharp-tags.scm
var cSharpTagQuery []byte

//go:embed tree-sitter-c-tags.scm
var cTagQuery []byte

//go:embed tree-sitter-cpp-tags.scm
var cppTagQuery []byte

//go:embed tree-sitter-dart-tags.scm
var dartTagQuery []byte

//go:embed tree-sitter-elisp-tags.scm
var elispTagQuery []byte

//go:embed tree-sitter-elixir-tags.scm
var elixirTagQuery []byte

//go:embed tree-sitter-elm-tags.scm
var elmTagQuery []byte

//go:embed tree-sitter-go-tags.scm
var goTagQuery []byte

//go:embed tree-sitter-java-tags.scm
var javaTagQuery []byte

//go:embed tree-sitter-javascript-tags.scm
var javascriptTagQuery []byte

//go:embed tree-sitter-ocaml-tags.scm
var ocamlTagQuery []byte

//go:embed tree-sitter-php-tags.scm
var phpTagQuery []byte

//go:embed tree-sitter-python-tags.scm
var pythonTagQuery []byte

//go:embed tree-sitter-ruby-tags.scm
var rubyTagQuery []byte

//go:embed tree-sitter-rust-tags.scm
var rustTagQuery []byte

//go:embed tree-sitter-typescript-tags.scm
var typescriptTagQuery []byte

// SitterLanguage is the language for the sitter queries
type SitterLanguage string

const (
	// CSharp is the language for C#
	CSharp SitterLanguage = "csharp"
	// C is the language for C
	C SitterLanguage = "c"
	// Cpp is the language for C++
	Cpp SitterLanguage = "cpp"
	// Dart is the language for Dart
	Dart SitterLanguage = "dart"
	// Elisp is the language for Elisp
	Elisp SitterLanguage = "elisp"
	// Elixir is the language for Elixir
	Elixir SitterLanguage = "elixir"
	// Elm is the language for Elm
	Elm SitterLanguage = "elm"
	// Go is the language for Go
	Go SitterLanguage = "go"
	// Java is the language for Java
	Java SitterLanguage = "java"
	// Javascript is the language for Javascript
	Javascript SitterLanguage = "javascript"
	// Ocaml is the language for Ocaml
	Ocaml SitterLanguage = "ocaml"
	// PHP is the language for PHP
	PHP SitterLanguage = "php"
	// Python is the language for Python
	Python SitterLanguage = "python"
	// Ruby is the language for Ruby
	Ruby SitterLanguage = "ruby"
	// Rust is the language for Rust
	Rust SitterLanguage = "rust"
	// Typescript is the language for Typescript
	Typescript SitterLanguage = "typescript"
)

// queries is a map of sitter queries for each language
var queries = map[SitterLanguage][]byte{
	CSharp:     cSharpTagQuery,
	C:          cTagQuery,
	Cpp:        cppTagQuery,
	Dart:       dartTagQuery,
	Elisp:      elispTagQuery,
	Elixir:     elixirTagQuery,
	Elm:        elmTagQuery,
	Go:         goTagQuery,
	Java:       javaTagQuery,
	Javascript: javascriptTagQuery,
	Ocaml:      ocamlTagQuery,
	PHP:        phpTagQuery,
	Python:     pythonTagQuery,
	Ruby:       rubyTagQuery,
	Rust:       rustTagQuery,
	Typescript: typescriptTagQuery,
}

// GetSitterQuery returns the sitter query for the given language
func GetSitterQuery(language SitterLanguage) ([]byte, error) {
	lang := SitterLanguage(language)
	query, ok := queries[lang]
	if !ok {
		return []byte{}, fmt.Errorf("language not supported")
	}
	return query, nil
}
