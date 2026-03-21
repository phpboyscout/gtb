// Package config provides configuration loading, merging, and access via the
// [Containable] interface backed by Viper.
//
// Configurations can be loaded from multiple sources — local files, embedded
// assets, environment variables, and command-line flags — and merged with
// deterministic precedence. Factory functions include [NewFilesContainer],
// [LoadFilesContainer], [NewReaderContainer], and [NewContainerFromViper].
//
// Type-safe accessors (GetString, GetInt, GetBool, GetDuration, GetTime, etc.)
// are exposed through [Containable]. For advanced use cases, [Containable.GetViper]
// provides direct access to the underlying Viper instance as an intentional
// power-user escape hatch.
//
// Hot-reload is supported via the [Observable] interface, which allows consumers
// to register callbacks that fire when configuration files change on disk.
package config
