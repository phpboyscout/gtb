// Package errorhandling provides structured, user-friendly error reporting for
// CLI tools built with GTB.
//
// The [ErrorHandler] interface offers Check (route through the reporting pipeline),
// Fatal (report and exit), Error (non-terminating report), and Warn methods.
// Errors are rendered with optional user-facing hints (via cockroachdb/errors
// WithHint/WithHintf) and help channel references (Slack, Teams) configured
// through [HelpConfig].
//
// Stack traces from cockroachdb/errors are automatically extracted and displayed
// in debug mode, providing rich diagnostic context without cluttering normal output.
package errorhandling
