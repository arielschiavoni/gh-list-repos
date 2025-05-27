package github

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/arielschiavoni/gh-lor/internal/utils"
	"github.com/cli/go-gh/v2/pkg/api"
	graphql "github.com/cli/shurcooL-graphql"
)

type GetUserRepositoriesQuery struct {
	User struct {
		Repositories Repositories `graphql:"repositories(ownerAffiliations: OWNER, first: $first, after: $cursor)"`
	} `graphql:"user(login: $username)"`
}

type GetOrgRepositoriesQuery struct {
	Organization struct {
		Repositories Repositories `graphql:"repositories(first: $first, after: $cursor, isArchived: $isArchived)"`
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

// Creates a unique repo key based on the name and topics of the repository
func (r Repository) Key(showTopics bool) string {
	key := r.NameWithOwner

	if showTopics && len(r.RepositoryTopics.Nodes) > 0 {
		topics := make([]string, 0, len(r.RepositoryTopics.Nodes))
		for _, t := range r.RepositoryTopics.Nodes {
			topics = append(topics, t.Topic.Name)
		}
		// Sort the topics alphabetically
		sort.Strings(topics)
		joinedTopicNames := fmt.Sprintf("[%s]", strings.Join(topics, ","))
		key = utils.AlignStrings(key, joinedTopicNames, 120)
	}

	return key
}

type RepositoryTopics struct {
	Nodes []struct {
		Topic struct {
			Name string
		}
	}
}

func GetUserRepositories(username string) ([]Repository, error) {
	log.Printf("[%s]: getting user repositories...\n", username)
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		log.Fatal(err)
	}

	var query GetUserRepositoriesQuery
	variables := map[string]any{
		"username": graphql.String(username),
		"first":    graphql.Int(100),
		"cursor":   (*graphql.String)(nil),
	}
	page := 1

	var repos []Repository
	for {
		log.Printf("[%s]: getting page %d...\n", username, page)

		err = client.Query("GetUserRepositories", &query, variables)
		if err != nil {
			log.Fatal(err)
		}

		if page == 1 {
			log.Printf("[%s]: this username has %d repos\n", username, query.User.Repositories.TotalCount)
		}

		repos = append(repos, query.User.Repositories.Nodes...)

		if !query.User.Repositories.PageInfo.HasNextPage {
			break
		}

		variables["cursor"] = graphql.String(query.User.Repositories.PageInfo.EndCursor)
		page += 1

	}

	var filteredRepos []Repository
	for _, repo := range repos {
		if !repo.IsArchived && !repo.IsFork {
			filteredRepos = append(filteredRepos, repo)
		}
	}

	return filteredRepos, nil
}

func GetOrgRepositories(org string) ([]Repository, error) {
	log.Printf("[%s]: getting org repositories...\n", org)
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		log.Fatal(err)
	}

	var query GetOrgRepositoriesQuery
	variables := map[string]any{
		"org":        graphql.String(org),
		"first":      graphql.Int(100),
		"cursor":     (*graphql.String)(nil),
		"isArchived": graphql.Boolean(false),
	}
	page := 1

	var repos []Repository
	for {
		log.Printf("[%s]: getting page %d...\n", org, page)

		err = client.Query("GetOrgRepositories", &query, variables)
		if err != nil {
			log.Fatal(err)
		}

		if page == 1 {
			log.Printf("[%s]: this org has %d repos\n", org, query.Organization.Repositories.TotalCount)
		}

		repos = append(repos, query.Organization.Repositories.Nodes...)

		if !query.Organization.Repositories.PageInfo.HasNextPage {
			break
		}

		variables["cursor"] = graphql.String(query.Organization.Repositories.PageInfo.EndCursor)
		page += 1

	}

	return repos, nil
}
