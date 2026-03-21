// Package vcs defines the version control system abstraction layer for querying
// releases and repository metadata across GitHub and GitLab backends.
//
// The sub-packages provide concrete implementations: github for the GitHub API
// (repository management, PRs, releases, asset downloads), gitlab for GitLab,
// release for the unified provider factory, and repo for repository URL parsing
// and metadata extraction.
package vcs
