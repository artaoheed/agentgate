package policy

import "regexp"

type Decision string

const (
	Allow  Decision = "allow"
	Redact Decision = "redact"
	Abort  Decision = "abort"
)

type Result struct {
	Decision Decision
	Reason   string
	// Matches holds the byte spans (in the input text) that triggered the
	// decision. Callers can use these to mask the offending text in-place,
	// so a subsequent re-evaluation of the same buffer doesn't re-fire.
	Matches [][]int
}

var (
	emailRegex = regexp.MustCompile(
		`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
	)
	phoneRegex = regexp.MustCompile(
		`\+?[0-9][0-9\-\s]{7,}[0-9]`,
	)
)

func EvaluatePII(text string) *Result {
	if m := emailRegex.FindAllStringIndex(text, -1); len(m) > 0 {
		return &Result{
			Decision: Abort,
			Reason:   "email_detected",
			Matches:  m,
		}
	}

	if m := phoneRegex.FindAllStringIndex(text, -1); len(m) > 0 {
		return &Result{
			Decision: Redact,
			Reason:   "phone_detected",
			Matches:  m,
		}
	}

	return nil
}

// Redact replaces each byte span in matches with a run of '*' of the same
// length. Spans are assumed to be non-overlapping and sorted (as returned by
// regexp.FindAllStringIndex).
func RedactSpans(text string, matches [][]int) string {
	if len(matches) == 0 {
		return text
	}
	b := []byte(text)
	for _, m := range matches {
		for i := m[0]; i < m[1] && i < len(b); i++ {
			b[i] = '*'
		}
	}
	return string(b)
}
