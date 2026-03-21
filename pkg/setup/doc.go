// Package setup provides initialisation helpers for GTB-based tools, including
// configuration directory bootstrapping, default config file creation, and
// self-update orchestration.
//
// The [Initialiser] interface supports a modular hook-based pattern for extending
// the init process — SSH key setup, authentication configuration, and custom
// post-init steps can be composed and ordered. Update checks use semantic version
// comparison against the configured release source (GitHub or GitLab).
package setup
