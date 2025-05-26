package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/arielschiavoni/gh-lor/internal/github"
)

func main() {
	// --- Logging Setup ---
	// Use the standard log location in the user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home directory: %v", err)
	}

	appDir := filepath.Join(homeDir, ".local", "share", "gh-lor")
	logFileName := filepath.Join(appDir, "logs.log")

	// Ensure the log directory exists
	if err := os.MkdirAll(appDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory %s: %v", appDir, err)
	}

	// Open the log file
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Exit if the file cannot be opened
		log.Fatalf("Error opening log file %s: %v", logFileName, err)
	}

	log.SetOutput(logFile)
	defer logFile.Close() // Ensure the file is closed when main exits

	log.Printf("Logging output to %s", logFileName)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile) // Add file and line number to log messages

	// --- End Logging Setup ---
	// Define flags
	usernamePtr := flag.String("username", "", "GitHub username to fetch repositories for")
	orgsPtr := flag.String("orgs", "", "Comma-separated list of GitHub organizations to fetch repositories for")
	// Updated flag description to mention the default path
	cacheFilePtr := flag.String("cache-file", "", "Path to a text file containing cached repository names (one per line). Defaults to ~/.local/share/gh-lor/cached-repos")

	// Parse flags
	flag.Parse()

	username := *usernamePtr
	orgString := *orgsPtr
	cacheFileFlagValue := *cacheFilePtr // Store the original value from the flag

	var orgs []string
	if orgString != "" {
		orgs = strings.Split(orgString, ",")
	}

	// Validate required arguments - check if *any* source is specified via flags
	// If username is empty AND orgs is empty AND the cache-file flag was NOT provided
	if username == "" && len(orgs) == 0 && cacheFileFlagValue == "" {
		fmt.Println("Usage: gh-lor [--username <username>] [--orgs <org1,org2,...>] [--cache-file <path>]")
		// Updated usage message for clarity regarding the default
		fmt.Println("\nAt least one of --username, --orgs, or --cache-file must be provided, or the default cache file path (~/.local/share/gh-lor/cached-repos) will be used if no flags are provided.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Determine the actual cache file path to use
	var cacheFile string
	if cacheFileFlagValue == "" {
		cacheFile = filepath.Join(appDir, "cached-repos")
		log.Printf("Using default cache file path: %s", cacheFile)
	} else {
		// Use the path provided by the flag
		cacheFile = cacheFileFlagValue
		// Ensure the directory for the user-provided path exists (unless it's just a filename)
		cacheDir := filepath.Dir(cacheFile)
		if cacheDir != "." {
			if err := os.MkdirAll(cacheDir, 0755); err != nil {
				log.Fatalf("Error creating directory for cache file %s: %v", cacheDir, err)
			}
		}
	}

	// Channel to stream repository names
	repoNameChannel := make(chan string)
	var wg sync.WaitGroup // Main wait group for all data sources

	// Use sync.Map for concurrent-safe tracking of seen repository names
	var seenRepos sync.Map // map[string]bool

	// Helper function to send a repo name if not already seen
	sendUniqueRepoName := func(name string) {
		if name != "" {
			_, loaded := seenRepos.LoadOrStore(name, true)
			if !loaded { // Only send if the key was not already present
				repoNameChannel <- name
			}
		}
	}

	// Goroutine to stream cached repositories from file
	// This goroutine runs if cacheFile is not empty (either default or flag value)
	if cacheFile != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Open the file - this will attempt to open the default or flag path
			file, err := os.Open(cacheFile)
			if err != nil {
				// Log a warning if the file doesn't exist or can't be opened, but don't stop execution
				// This allows the program to run with just GitHub sources, or start with an empty cache file
				log.Printf("Warning: Error opening cache file %s: %v", cacheFile, err)
				return // Don't stop the program, just skip cache reading
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				repoName := strings.TrimSpace(scanner.Text())
				sendUniqueRepoName(repoName)
			}
			if err := scanner.Err(); err != nil {
				log.Printf("Warning: Error reading cache file %s: %v", cacheFile, err)
			}
		}()
	}

	// Goroutine to fetch and stream repositories from GitHub API
	if username != "" || len(orgs) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done() // Decrement main wg when this goroutine finishes

			var orgWG sync.WaitGroup // Wait group for organization fetches

			// Get user repositories if username is provided
			if username != "" {
				userRepos, err := github.GetUserRepositories(username)
				if err != nil {
					log.Printf("Error getting user repositories for %s: %v", username, err)
				} else {
					for _, repo := range userRepos {
						sendUniqueRepoName(repo.NameWithOwner)
					}
				}
			}

			// Get organization repositories if orgs are provided (in parallel)
			if len(orgs) > 0 {
				for _, org := range orgs {
					orgWG.Add(1)                 // Increment org wg for each org goroutine
					go func(currentOrg string) { // Launch new goroutine for each org
						defer orgWG.Done() // Decrement org wg when this org goroutine finishes

						repos, err := github.GetOrgRepositories(currentOrg)
						if err != nil {
							// Log error but continue with other orgs
							// log.Printf("Warning: Error getting organization repositories for %s: %v", currentOrg, err)
							return // Stop processing this org's results on error
						}
						for _, repo := range repos {
							sendUniqueRepoName(repo.NameWithOwner)
						}
					}(org) // Pass the current org value to the goroutine
				}
				orgWG.Wait() // Wait for all org goroutines to complete
			}
		}()
	}

	// Goroutine to close the channel when all data source workers are done
	go func() {
		// Wait for the file goroutine (if active) and the API goroutine (if active)
		wg.Wait()
		close(repoNameChannel)
	}()

	var collectedRepos []string
	// Stream results from the channel to standard output (e.g., fzf)
	for repoName := range repoNameChannel {
		fmt.Println(repoName)
		collectedRepos = append(collectedRepos, repoName)
	}

	// Implement saving the combined unique results to the cache file at the end
	log.Printf("Saving %d unique repositories to cache file: %s", len(collectedRepos), cacheFile)
	err = os.WriteFile(cacheFile, []byte(strings.Join(collectedRepos, "\n")+"\n"), 0644)
	if err != nil {
		log.Printf("Error writing cache file %s: %v", cacheFile, err)
	}
}
