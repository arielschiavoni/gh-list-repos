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
	// Ensure the file is closed when main exits
	defer logFile.Close()

	log.Printf("Logging output to %s", logFileName)
	// Add file and line number to log messages
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Define flags
	usernamePtr := flag.String("username", "", "GitHub username to fetch repositories for")
	orgsPtr := flag.String("orgs", "", "Comma-separated list of GitHub organizations to fetch repositories for")
	showTopicsPtr := flag.Bool("show-topics", false, "Shows repository topics")

	// Parse flags
	flag.Parse()

	username := *usernamePtr
	orgString := *orgsPtr
	showTopics := *showTopicsPtr

	var orgs []string
	if orgString != "" {
		orgs = strings.Split(orgString, ",")
	}

	// Print help if orgs and username are not specified
	if username == "" && len(orgs) == 0 {
		fmt.Println("Usage: gh-lor [--username <username>] [--orgs <org1,org2,...>] [--showTopics] [--cache-file <path>]")
		fmt.Println("\nAt least one of --username or --orgs must be provided")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Channel to stream repository keys (name + topics)
	repoKeyChannel := make(chan string)

	// Use sync.Map for concurrent-safe tracking of seen repository names
	var seenRepos sync.Map

	// Helper function to send a repo key if not already seen
	sendUniqueRepoKey := func(key string) {
		if key != "" {
			_, loaded := seenRepos.LoadOrStore(key, true)
			// Only send if the key was not already present
			if !loaded {
				repoKeyChannel <- key
			}
		}
	}

	// Determine the actual cache file path to use
	cacheFile := filepath.Join(appDir, "repos")
	if showTopics {
		cacheFile += "_with_topics"
	}
	// Ensure the directory for the user-provided path exists (unless it's just a filename)
	cacheDir := filepath.Dir(cacheFile)
	if cacheDir != "." {
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			log.Fatalf("Error creating directory for cache file %s: %v", cacheDir, err)
		}
	}

	// Main wait group for all data sources
	var wg sync.WaitGroup
	wg.Add(1)

	// Goroutine to stream cached repositories from file
	go func() {
		defer wg.Done()
		// Open the file - this will attempt to open the default or flag path
		file, err := os.Open(cacheFile)
		if err != nil {
			// Log a warning if the file doesn't exist or can't be opened, but don't stop execution
			// This allows the program to run with just GitHub sources, or start with an empty cache file
			log.Printf("Warning: Error opening cache file %s: %v", cacheFile, err)
			// Don't stop the program, just skip cache reading
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			repoKey := strings.TrimSpace(scanner.Text())
			sendUniqueRepoKey(repoKey)
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Warning: Error reading cache file %s: %v", cacheFile, err)
		}
	}()

	if username != "" || len(orgs) > 0 {
		wg.Add(1)

		// Goroutine to fetch and stream repositories from GitHub API
		go func() {
			// Decrement main wg when this goroutine finishes
			defer wg.Done()

			// Get user repositories if username is provided
			if username != "" {
				userRepos, err := github.GetUserRepositories(username)
				if err != nil {
					log.Printf("Error getting user repositories for %s: %v", username, err)
				} else {
					for _, repo := range userRepos {
						sendUniqueRepoKey(repo.Key(showTopics))
					}
				}
			}

			// Wait group for organization fetches
			var orgWG sync.WaitGroup

			// Get organization repositories if orgs are provided (in parallel)
			if len(orgs) > 0 {
				for _, org := range orgs {
					// Increment org wg for each org goroutine
					orgWG.Add(1)

					// Launch new goroutine for each org
					go func(currentOrg string) {
						// Decrement org wg when this org goroutine finishes
						defer orgWG.Done()

						repos, err := github.GetOrgRepositories(currentOrg)
						if err != nil {
							// Log error but continue with other orgs
							log.Printf("Warning: Error getting organization repositories for %s: %v", currentOrg, err)
							// Stop processing this org's results on error
							return
						}
						for _, repo := range repos {
							sendUniqueRepoKey(repo.Key(showTopics))
						}
						// Pass the current org value to the goroutine
					}(org)
				}
				orgWG.Wait() // Wait for all org goroutines to complete
			}
		}()
	}

	// Goroutine to close the channel when all data source workers are done
	go func() {
		// Wait for the file goroutine (if active) and the API goroutine (if active)
		wg.Wait()
		close(repoKeyChannel)
	}()

	var collectedRepos []string
	// Stream results from the channel to standard output (e.g., fzf)
	for repoName := range repoKeyChannel {
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
