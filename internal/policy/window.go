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
