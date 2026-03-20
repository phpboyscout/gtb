package errorhandling

import (
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
)

// extractStackTrace returns the stack/chain detail from err, stripped of the
// leading error message that %+v always repeats. When no extra information
// exists (e.g. a plain fmt.Errorf), it returns a note to that effect so the
// caller can clearly distinguish "no trace captured" from a trace that happens
// to look identical to the message.
func extractStackTrace(err error) string {
	verbose := fmt.Sprintf("%+v", err)
	msg := err.Error()

	// cockroachdb %+v output starts with the error message followed by a
	// newline and the encoded cause chain / stack frames. Strip the prefix so
	// the stacktrace field contains only the useful detail.
	after, found := strings.CutPrefix(verbose, msg)
	if !found || strings.TrimSpace(after) == "" {
		return "(no stack trace captured)"
	}

	return cleanStackTrace(strings.TrimPrefix(after, "\n"))
}

// cleanStackTrace normalises cockroachdb's %+v stack output so it renders
// cleanly inside charmbracelet/log's multi-line value formatter.
//
// cockroachdb prefixes every stack-frame line with "  | ", which collides with
// charmbracelet/log's own "│ " wrapper to produce a double-pipe. It also uses
// a tab character before file paths which charmbracelet/log renders as a
// literal \t escape sequence. Both are replaced with plain spaces here.
func cleanStackTrace(trace string) string {
	lines := strings.Split(trace, "\n")

	for i, line := range lines {
		rest, ok := strings.CutPrefix(line, "  | ")
		if !ok {
			continue
		}

		// Replace actual tab characters and the literal two-char \t sequence
		// that cockroachdb sometimes emits before file paths.
		rest = strings.ReplaceAll(rest, "\t", "  ")
		rest = strings.ReplaceAll(rest, `\t`, "  ")
		lines[i] = "    " + rest
	}

	return strings.Join(lines, "\n")
}

// WithUserHint attaches a user-facing recovery suggestion to an error.
func WithUserHint(err error, hint string) error {
	return errors.WithHint(err, hint)
}

// WithUserHintf attaches a formatted user-facing recovery suggestion.
func WithUserHintf(err error, format string, args ...any) error {
	return errors.WithHintf(err, format, args...)
}

// WrapWithHint wraps an error with a message and attaches a user-facing hint.
func WrapWithHint(err error, msg string, hint string) error {
	return errors.WithHint(errors.Wrap(err, msg), hint)
}

// NewAssertionFailure creates an error denoting a programming bug.
func NewAssertionFailure(format string, args ...any) error {
	return errors.AssertionFailedf(format, args...)
}
