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
	if emailRegex.MatchString(text) {
		return &Result{
			Decision: Abort,
			Reason:   "email_detected",
		}
	}

	if phoneRegex.MatchString(text) {
		return &Result{
			Decision: Redact,
			Reason:   "phone_detected",
		}
	}

	return nil
}
