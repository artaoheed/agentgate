package policy

import "testing"

func TestEvaluatePII(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     Decision
		wantNil  bool
		wantHits int
	}{
		{"clean text", "hello world, nothing sensitive here", "", true, 0},
		{"empty", "", "", true, 0},
		{"single email aborts", "contact me at jane@example.com", Abort, false, 1},
		{"email beats phone", "jane@example.com or +1 555 123 4567", Abort, false, 1},
		{"phone redacts", "call +1 555 123 4567 please", Redact, false, 1},
		{"multiple phones", "+1 555 123 4567 or +44 20 7946 0958", Redact, false, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluatePII(tc.input)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil result, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %s, got nil", tc.want)
			}
			if got.Decision != tc.want {
				t.Errorf("decision = %s, want %s", got.Decision, tc.want)
			}
			if len(got.Matches) != tc.wantHits {
				t.Errorf("matches = %d, want %d", len(got.Matches), tc.wantHits)
			}
		})
	}
}

func TestRedactSpans(t *testing.T) {
	in := "call +1 555 123 4567 today"
	res := EvaluatePII(in)
	if res == nil || res.Decision != Redact {
		t.Fatalf("setup: expected redact result, got %+v", res)
	}
	out := RedactSpans(in, res.Matches)
	// Re-evaluating the masked text must not re-fire.
	if again := EvaluatePII(out); again != nil {
		t.Errorf("expected no PII after redact, got %+v in %q", again, out)
	}
	// Length must be preserved (in-place mask).
	if len(out) != len(in) {
		t.Errorf("length changed: %d -> %d", len(in), len(out))
	}
}

func TestRedactSpans_NoMatches(t *testing.T) {
	in := "nothing to redact"
	if got := RedactSpans(in, nil); got != in {
		t.Errorf("unexpected change: %q", got)
	}
}
