package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/arielschiavoni/gh-list-repos/internal/github"
)

func main() {
	// Use the standard log location in the user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get user home directory: %v", err)
	}

	appDir := filepath.Join(homeDir, ".local", "share", "gh-list-repos")
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
	// Ensure the file is closed when main exits
	defer logFile.Close()

	log.Printf("Logging output to %s", logFileName)
	// Add file and line number to log messages
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Define flags
	usernamePtr := flag.String("username", "", "GitHub username to fetch repositories from")
	orgsPtr := flag.String("orgs", "", "Comma-separated list of GitHub organizations to fetch repositories from")
	noArchivedPtr := flag.Bool("no-archived", false, "Excludes archived repositories")
	noForkPtr := flag.Bool("no-fork", false, "Excludes forked repositories")

	// Parse flags
	flag.Parse()

	username := *usernamePtr
	orgString := *orgsPtr
	noArchived := *noArchivedPtr
	noFork := *noForkPtr

	var orgs []string
	if orgString != "" {
		orgs = strings.Split(orgString, ",")
	}

	// Print help if orgs and username are not specified
	if username == "" && len(orgs) == 0 {
		fmt.Println("Usage: gh list-repos [-username <username>] [-orgs <org1,org2,...>] [-no-archived] [-no-fork]")
		fmt.Println("\nAt least one of --username or --orgs must be provided")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Channel to send repository lines to
	repoLinesChannel := make(chan string)

	// Main wait group for all data sources
	var wg sync.WaitGroup

	if username != "" || len(orgs) > 0 {
		wg.Add(1)

		// Goroutine to fetch and stream repositories from GitHub API
		go func() {
			// Decrement main wg when this goroutine finishes
			defer wg.Done()

			// Wait group for user and organization fetches to run in parallel
			var fetchWG sync.WaitGroup

			// Get user repositories if username is provided (in parallel)
			if username != "" {
				fetchWG.Add(1)

				// Launch new goroutine for username
				go func() {
					defer fetchWG.Done()

					err := github.ProcessUserRepositories(username, noArchived, noFork, repoLinesChannel)
					if err != nil {
						log.Printf("Error getting user repositories for %s: %v", username, err)
					}
				}()
			}

			// Get organization repositories if orgs are provided
			if len(orgs) > 0 {
				for _, org := range orgs {
					fetchWG.Add(1)

					// Launch new goroutine for each org
					go func(currentOrg string) {
						// Decrement fetch wg when this org goroutine finishes
						defer fetchWG.Done()

						err := github.ProcessOrgRepositories(currentOrg, noArchived, noFork, repoLinesChannel)
						if err != nil {
							// Log error but continue with other orgs
							log.Printf("Warning: Error getting organization repositories for %s: %v", currentOrg, err)
						}
						// Pass the current org value to the goroutine
					}(org)
				}
			}

			// Wait for all user and org goroutines to complete
			fetchWG.Wait()
		}()
	}

	// Goroutine to close the channel when all data source workers are done
	go func() {
		// Wait for the API goroutine (if active)
		wg.Wait()
		close(repoLinesChannel)
	}()

	var repos []string
	// Stream results from the channel to standard output (e.g., fzf)
	for repoName := range repoLinesChannel {
		fmt.Println(repoName)
		repos = append(repos, repoName)
	}

	// if isFileCacheEnabled {
	// 	// Implement saving the combined unique results to the cache file at the end
	// 	log.Printf("Saving %d unique repositories to cache file: %s", len(repos), cacheFile)
	//
	// 	err = os.WriteFile(cacheFile, []byte(strings.Join(repos, "\n")+"\n"), 0644)
	// 	if err != nil {
	// 		log.Printf("Error writing cache file %s: %v", cacheFile, err)
	// 	}
	// }
}
