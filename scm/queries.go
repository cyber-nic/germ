package scm

import "fmt"

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
var queries = map[SitterLanguage]string{
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
func GetSitterQuery(language SitterLanguage) (string, error) {
	lang := SitterLanguage(language)
	query, ok := queries[lang]
	if !ok {
		return "", fmt.Errorf("language not supported")
	}
	return query, nil
}
