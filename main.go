package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/arielschiavoni/gh-lor/internal/github"
)

func main() {
	fmt.Println("Welcome to the gh-lor extension!")

	// Define flags
	usernamePtr := flag.String("username", "", "GitHub username to fetch repositories for")
	orgsPtr := flag.String("orgs", "", "Comma-separated list of GitHub organizations to fetch repositories for")

	// Parse flags
	flag.Parse()

	username := *usernamePtr
	orgString := *orgsPtr
	var orgs []string
	if orgString != "" {
		orgs = strings.Split(orgString, ",")
	}

	// Validate required arguments
	if username == "" && len(orgs) == 0 {
		fmt.Println("Usage: gh-lor --username <username> [--orgs <org1,org2,...>]")
		fmt.Println("Or:    gh-lor [--username <username>] --orgs <org1,org2,...>")
		fmt.Println("\nAt least one of --username or --orgs must be provided.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	var allRepos []github.Repository

	// Get user repositories if username is provided
	if username != "" {
		userRepos, err := github.GetUserRepositories(username)
		if err != nil {
			log.Fatalf("Error getting user repositories for %s: %v", username, err)
		}
		allRepos = append(allRepos, userRepos...)
	}

	// Get organization repositories if orgs are provided
	if len(orgs) > 0 {
		for _, org := range orgs {
			repos, err := github.GetOrgRepositories(org)
			if err != nil {
				// Log error but continue with other orgs
				log.Printf("Warning: Error getting organization repositories for %s: %v", org, err)
				continue
			}
			allRepos = append(allRepos, repos...)
		}
	}

	fmt.Printf("Total repos found: %d\n", len(allRepos))

	// Sort repositories if needed (optional)
	sort.Slice(allRepos, func(i, j int) bool {
		return allRepos[i].NameWithOwner < allRepos[j].NameWithOwner
	})

	for _, repo := range allRepos {
		fmt.Println(repo.NameWithOwner)
	}
}
