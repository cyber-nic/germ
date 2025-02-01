# Graph Entity Repo Map (germ)

_A Go library for building repository code maps using AST/CST. Particularly usefuel with AI-powered coding assistants._

<p align="center">
  <img height="256" src="./germ.svg">
</p>

> [!CAUTION]
> This project is still very much in development

## Overview

**Germ** (Graph Entity Repo Map) is a Go library designed to analyze and map code repositories by leveraging **Abstract Syntax Trees (AST)** and **Concrete Syntax Trees (CST)**. It enables AI-powered tools to understand code structures, dependencies, and relationships efficiently.

## Features

- **Graph-based repository mapping**: Extracts entities and relationships from source code.
- **AST/CST parsing**: Supports deep syntax analysis for code intelligence.
- **AI-friendly representation**: Generates structured data suitable for AI coding assistants.
- **Language support**: Primarily focused on Go, with extensibility for other languages.
- **Efficient and scalable**: Designed for large repositories with minimal performance overhead.

## Installation

```sh
go get github.com/cyber-nic/germ
```

## Usage

### Exlucding Files and Folers

Use a .gitignore or create a git-compatible .astignore. Alternatily copy the .astignore from this repo into yours.

### Example

See `cmd/main.go` for a working example.

## License

[MIT License](LICENSE.md)
