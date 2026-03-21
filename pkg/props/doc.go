// Package props defines the Props dependency container, the central type-safe
// dependency injection mechanism used throughout GTB.
//
// Props is a concrete struct that aggregates tool metadata, logger, configuration,
// filesystem (afero.Fs), embedded assets, version info, and error handler into a
// single value threaded through commands and services. It replaces context.Context-based
// DI with compile-time type safety.
//
// The package also provides the feature flag system via [SetFeatures], [Enable], and
// [Disable] functional options, controlling which built-in commands (update, init,
// docs, mcp) are registered at startup. Tool release source metadata for GitHub and
// GitLab backends is managed through [ReleaseSource].
package props
