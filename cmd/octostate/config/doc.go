// Package config provides GitOps configuration commands for octostate.
//
// These commands operate on local desired-state files. Some commands, such as
// `config validate`, are fully offline, while others, such as `config plan`,
// `config apply`, and `config sync-from-live`, read or reconcile live GitHub
// state and therefore require GitHub API access and authentication.
package config
