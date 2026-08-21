package automation

import (
	"errors"
	"os/exec"
)

// App combines a window selector with an optional launch command.
type App struct {
	Query   WindowQuery
	Command string
	Args    []string
}

func (a App) Toggle() (Window, error) { return ToggleWindow(a.Query) }

// ToggleOrStart toggles an existing window, or starts Command when absent.
func (a App) ToggleOrStart() (Window, *exec.Cmd, error) {
	w, e := FindWindow(a.Query)
	if e == nil {
		return w, nil, w.Toggle()
	}
	if !errors.Is(e, ErrNotFound) {
		return Window{}, nil, e
	}
	if a.Command == "" {
		return Window{}, nil, ErrNotFound
	}
	cmd := exec.Command(a.Command, a.Args...)
	if e := cmd.Start(); e != nil {
		return Window{}, nil, e
	}
	return Window{}, cmd, nil
}

func (w Window) Center(bounds Rect) error {
	r, e := w.Rect()
	if e != nil {
		return e
	}
	return w.Move(Point{bounds.Left + (bounds.Width()-r.Width())/2, bounds.Top + (bounds.Height()-r.Height())/2})
}

type SnapPosition uint8

const (
	SnapCenter SnapPosition = iota
	SnapLeft
	SnapRight
	SnapTop
	SnapBottom
)

func (w Window) Snap(bounds Rect, p SnapPosition) error {
	r, e := w.Rect()
	if e != nil {
		return e
	}
	x, y := r.Left, r.Top
	switch p {
	case SnapCenter:
		x = bounds.Left + (bounds.Width()-r.Width())/2
		y = bounds.Top + (bounds.Height()-r.Height())/2
	case SnapLeft:
		x = bounds.Left
	case SnapRight:
		x = bounds.Right - r.Width()
	case SnapTop:
		y = bounds.Top
	case SnapBottom:
		y = bounds.Bottom - r.Height()
	}
	return w.Move(Point{x, y})
}
