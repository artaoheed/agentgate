package policy

import (
	"strings"
	"testing"
)

func TestRollingWindow_AddAndCap(t *testing.T) {
	w := NewRollingWindow(10)
	w.Add("abcdef")
	if w.Text() != "abcdef" {
		t.Fatalf("text = %q, want abcdef", w.Text())
	}
	w.Add("ghijkl")
	// Buffer should hold only the last 10 bytes.
	if got := w.Text(); got != "cdefghijkl" {
		t.Errorf("text = %q, want cdefghijkl", got)
	}
}

func TestRollingWindow_Mask_PreventsReFire(t *testing.T) {
	w := NewRollingWindow(300)
	w.Add("ring me at +1 555 123 4567 please")

	res := EvaluatePII(w.Text())
	if res == nil || res.Decision != Redact {
		t.Fatalf("expected redact, got %+v", res)
	}
	w.Mask(res.Matches)

	if again := EvaluatePII(w.Text()); again != nil {
		t.Errorf("redact should be sticky, got re-fire: %+v in %q", again, w.Text())
	}
	if !strings.Contains(w.Text(), "*") {
		t.Errorf("expected masked chars in buffer, got %q", w.Text())
	}
}
