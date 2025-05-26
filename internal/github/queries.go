package github

import (
	"log"

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
	NameWithOwner string
	IsFork        bool
	IsArchived    bool
}

func GetUserRepositories(username string) ([]Repository, error) {
	log.Printf("Getting user repositories for %s ...\n", username)
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
		log.Printf("Getting page %d for user %s ...\n", page, username)

		err = client.Query("GetUserRepositories", &query, variables)
		if err != nil {
			log.Fatal(err)
		}

		if page == 1 {
			log.Printf("Got the first page, TotalCount: %d\n", query.User.Repositories.TotalCount)
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
	log.Printf("Getting org repositories for %s ...\n", org)
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
		log.Printf("Getting page %d for org %s ...\n", page, org)

		err = client.Query("GetOrgRepositories", &query, variables)
		if err != nil {
			log.Fatal(err)
		}

		if page == 1 {
			log.Printf("Got the first page, TotalCount: %d\n", query.Organization.Repositories.TotalCount)
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
