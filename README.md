# gh-list-repos

A GitHub (gh) CLI extension to list repositories from a user or multiple organizations


## 📦 Installation

1. Install the `gh` CLI - see the [installation](https://github.com/cli/cli#installation)

   _Installation requires a minimum version (2.0.0) of the GitHub CLI that supports extensions._

2. Install this extension:

   ```sh
   gh extension install arielschiavoni/gh-list-repos
   ```


## ⚡️ How to use

Run

```sh
gh list-repos
```

```
Usage: gh list-repos [-username <username>] [-orgs <org1,org2,...>] [-no-archived] [-no-fork]

At least one of --username or --orgs must be provided
  -no-archived
        Excludes archived repositories
  -no-fork
        Excludes forked repositories
  -orgs string
        Comma-separated list of GitHub organizations to fetch repositories from
  -username string
        GitHub username to fetch repositories from
```

Example combined with [fzf](https://github.com/junegunn/fzf)

```shell
gh list-repos -username arielschiavoni | fzf
```
