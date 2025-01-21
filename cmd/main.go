package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	orb "github.com/cyber-nic/orb"
)

// findGitRoot walks upward from the given path until
// it finds a directory containing a ".git" folder.
func findGitRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("could not get absolute path of %q: %w", start, err)
	}

	for {
		// Does ".git" exist here?
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err == nil && info.IsDir() {
			// Found .git
			return current, nil
		}

		// If we can't go higher, stop
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("no .git folder found starting from %q and up", start)
}

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s [path-to-file-or-dir]\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	trace := true
	ConfigLogging(&trace, &trace)

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
	root, err := findGitRoot(absPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Error finding .git")
	}

	// 3. Build the RepoMap
	//    Make sure you have imported and can reference your repomap code. For example:
	//    import "github.com/yourname/yourrepo/repomap"
	//    or if it's in the same module, something like "myproject/repomap"
	rm := orb.NewRepoMap(
		1024,             // maxMapTokens
		root,             // pass the discovered root
		&orb.ModelStub{}, // or your real model
		"",               // repoContentPrefix
		false,            // verbose
		16000,            // maxContextWindow
		8,                // mapMulNoFiles
		"auto",           // refresh
	)

	// 4. Decide which files are "chat files" vs. "other files"
	//    This part depends on your usage pattern. For a simple example:
	//    - If the input is a single file, treat that as the 'chat file'
	//    - If the input is a directory, gather all files from that directory as 'chat files'
	//    - Then, optionally, gather other files from the entire repo if you want a full map.

	var allFiles []string
	// var chatFiles []string
	var otherFiles []string

	// If it's a directory, gather all files there as "chat files"
	info, err := os.Stat(absPath)
	if err == nil && info.IsDir() {
		allFiles = orb.FindSrcFiles(absPath) // A helper that gathers files (like in your repomap code)
	} else {
		// Single file
		allFiles = []string{absPath}
	}

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

	// 5. Actually call GetRepoMap
	mentionedFnames := map[string]bool{}
	mentionedIdents := map[string]bool{}
	forceRefresh := false

	repoMapOutput := rm.GetRepoMap(
		allFiles,
		otherFiles,
		mentionedFnames,
		mentionedIdents,
		forceRefresh,
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

	// add ORB_LOG env variable to set log level
	if logLevel, ok := os.LookupEnv("ORB_LOG"); ok {
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
