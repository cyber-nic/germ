package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/cyber-nic/germ"
)

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [path-to-file-or-dir]\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	trace := false
	debug := false
	ConfigLogging(&trace, &debug)

	inputPath := "."
	if len(os.Args) == 2 {
		inputPath = os.Args[1]
	}

	// 1. Get the path argument
	absPath, err := filepath.Abs(inputPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Error getting absolute path")
	}

	// 2. Find the root of the git repo
	root, err := germ.FindGitRoot(absPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Error finding .git")
	}

	// 3. Build the RepoMap
	//    Make sure you have imported and can reference your repomap code. For example:
	//    import "github.com/yourname/yourrepo/repomap"
	//    or if it's in the same module, something like "myproject/repomap"
	rm := germ.NewRepoMap(
		root,              // pass the discovered root
		&germ.ModelStub{}, // or your real model
		germ.WithLogLevel(int(zerolog.DebugLevel)),
	)

	// 4. Decide which files are "chat files" vs. "other files"
	//    This part depends on your usage pattern. For a simple example:
	//    - If the input is a single file, treat that as the 'chat file'
	//    - If the input is a directory, gather all files from that directory as 'chat files'
	//    - Then, optionally, gather other files from the entire repo if you want a full map.

	// var chatFiles []string
	var otherFiles []string

	allFiles, treeMap := rm.GetRepoFiles(absPath)

	fmt.Println(treeMap)

	// chatSet := make(map[string]bool)
	// for _, cf := range chatFiles {
	// 	chatSet[filepath.Clean(cf)] = true
	// }

	// for _, f := range allFiles {
	// 	cleanF := filepath.Clean(f)
	// 	if !chatSet[cleanF] {
	// 		otherFiles = append(otherFiles, cleanF)
	// 	}
	// }

	// for f, _ := range otherFiles {
	// 	fmt.Printf("- %s\n", f)
	// }

	// for f, _ := range chatSet {
	// 	fmt.Printf("- %s\n", f)
	// }

	// 5. Generate Repo Map
	mentionedFnames := map[string]bool{}
	mentionedIdents := map[string]bool{}

	repoMapOutput := rm.Generate(
		allFiles,
		otherFiles,
		mentionedFnames,
		mentionedIdents,
	)

	if repoMapOutput == "" {
		fmt.Println("Empty Repo Map")
		return
	}

	fmt.Println(repoMapOutput)
}

// ConfigLogging configures the logging level and format
func ConfigLogging(trace, debug *bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	log.Logger = log.With().Caller().Logger()

	if *trace {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
		log.Debug().Msg("Trace logging enabled")
		return
	}

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Debug().Msg("Debug logging enabled")
		return
	}

	// add GERM_LOG env variable to set log level
	if logLevel, ok := os.LookupEnv("GERM_LOG"); ok {
		switch logLevel {
		case "debug":
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			*debug = true
			log.Debug().Msg("debug logging enabled")
		case "trace":
			zerolog.SetGlobalLevel(zerolog.TraceLevel)
			*trace = true
			*debug = true
			log.Trace().Msg("trace logging enabled")
		case "error":
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		default:
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
			log.Warn().Msgf("Invalid log level: %s", logLevel)
		}
		return
	}

	// default log level
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}
