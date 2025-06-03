package github

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/arielschiavoni/gh-list-repos/internal/utils"
	"github.com/cli/go-gh/v2/pkg/api"
	graphql "github.com/cli/shurcooL-graphql"
)

const pageSize = 100
const maxLineWidth = 150

type GetUserRepositoriesQuery struct {
	User struct {
		Repositories Repositories `graphql:"repositories(ownerAffiliations: OWNER, first: $first, after: $cursor, isArchived: $isArchived, isFork: $isFork)"`
	} `graphql:"user(login: $username)"`
}

type GetOrgRepositoriesQuery struct {
	Organization struct {
		Repositories Repositories `graphql:"repositories(first: $first, after: $cursor, isArchived: $isArchived, isFork: $isFork)"`
	} `graphql:"organization(login: $org)"`
}

type Repositories struct {
	TotalCount int
	Nodes      []Repository
	PageInfo   struct {
		EndCursor   string
		HasNextPage bool
	}
}

type Repository struct {
	NameWithOwner    string
	IsFork           bool
	IsArchived       bool
	RepositoryTopics RepositoryTopics `graphql:"repositoryTopics(first: 5)"`
}

type RepositoryTopics struct {
	Nodes []struct {
		Topic struct {
			Name string
		}
	}
}

// Creates a unique repo description line based on the name and other repository details like topics
func (r Repository) Line() string {
	// the key is composed of a "left" side (NameWithOwner) and right side (IsArchived, IsFork, and topics)
	left := r.NameWithOwner

	var right []string

	// Add warning color if the repository is archived
	if r.IsArchived {
		right = append(right, "archived")
	}

	if r.IsFork {
		right = append(right, "fork")
	}

	if len(r.RepositoryTopics.Nodes) > 0 {
		topics := make([]string, 0, len(r.RepositoryTopics.Nodes))
		for _, node := range r.RepositoryTopics.Nodes {
			topics = append(topics, node.Topic.Name)
		}
		// Sort the topics alphabetically
		sort.Strings(topics)
		right = append(right, fmt.Sprintf("[%s]", strings.Join(topics, ",")))

	}

	// if the right part is empty then return only the left side
	if len(right) == 0 {
		return left
	}

	// if the right part contains either "archived", "fork" or a list of topics
	// then it needs to be aligned to right side and the available space determined by maxLineWidth
	// needs to be filled with spaces
	return utils.AlignStrings(left, strings.Join(right, " | "), maxLineWidth)
}

func ProcessUserRepositories(username string, noArchived bool, noFork bool, repoLinesChannel chan string) error {
	log.Printf("[%s]: getting repositories...\n", username)
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		log.Fatal(err)
	}

	var query GetUserRepositoriesQuery
	variables := map[string]any{
		"username":   graphql.String(username),
		"first":      graphql.Int(pageSize),
		"cursor":     (*graphql.String)(nil),
		"isArchived": (*graphql.Boolean)(nil),
		"isFork":     (*graphql.Boolean)(nil),
	}

	if noArchived {
		variables["isArchived"] = graphql.Boolean(false)
	}

	if noFork {
		variables["isFork"] = graphql.Boolean(false)
	}

	page := 1

	for {
		log.Printf("[%s]: getting page %d...\n", username, page)

		err = client.Query("GetUserRepositories", &query, variables)
		if err != nil {
			log.Fatal(err)
		}

		if page == 1 {
			log.Printf("[%s]: has %d repos\n", username, query.User.Repositories.TotalCount)
		}

		for _, repo := range query.User.Repositories.Nodes {
			// send repo line to channel
			repoLinesChannel <- repo.Line()
		}

		if !query.User.Repositories.PageInfo.HasNextPage {
			break
		}

		variables["cursor"] = graphql.String(query.User.Repositories.PageInfo.EndCursor)
		page += 1

	}

	return nil
}

func ProcessOrgRepositories(org string, noArchived bool, noFork bool, repoLinesChannel chan string) error {
	log.Printf("[%s]: getting repositories...\n", org)
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		log.Fatal(err)
	}

	var query GetOrgRepositoriesQuery
	variables := map[string]any{
		"org":        graphql.String(org),
		"first":      graphql.Int(pageSize),
		"cursor":     (*graphql.String)(nil),
		"isArchived": (*graphql.Boolean)(nil),
		"isFork":     (*graphql.Boolean)(nil),
	}

	if noArchived {
		variables["isArchived"] = graphql.Boolean(false)
	}

	if noFork {
		variables["isFork"] = graphql.Boolean(false)
	}

	page := 1

	for {
		log.Printf("[%s]: getting page %d...\n", org, page)

		err = client.Query("GetOrgRepositories", &query, variables)
		if err != nil {
			log.Fatal(err)
		}

		if page == 1 {
			log.Printf("[%s]: has %d repos\n", org, query.Organization.Repositories.TotalCount)
		}

		for _, repo := range query.Organization.Repositories.Nodes {
			// send repo line to channel
			repoLinesChannel <- repo.Line()
		}

		if !query.Organization.Repositories.PageInfo.HasNextPage {
			break
		}

		variables["cursor"] = graphql.String(query.Organization.Repositories.PageInfo.EndCursor)
		page += 1

	}

	return nil
}
