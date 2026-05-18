package policy

type RollingWindow struct {
	size int
	buf  string
}

func NewRollingWindow(size int) *RollingWindow {
	return &RollingWindow{size: size}
}

func (w *RollingWindow) Add(text string) {
	w.buf += text
	if len(w.buf) > w.size {
		w.buf = w.buf[len(w.buf)-w.size:]
	}
}

func (w *RollingWindow) Text() string {
	return w.buf
}

// Mask replaces the given byte spans in the current buffer with '*'. Use
// this after a Redact decision so the same match isn't re-detected on the
// next evaluation.
func (w *RollingWindow) Mask(matches [][]int) {
	w.buf = RedactSpans(w.buf, matches)
}
