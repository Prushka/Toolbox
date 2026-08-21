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
	args := append([]string(nil), a.Args...)
	cmd := exec.Command(a.Command, args...)
	if e := cmd.Start(); e != nil {
		return Window{}, nil, e
	}
	return Window{}, cmd, nil
}

func (w Window) Center(bounds Rect) error {
	if bounds.Empty() {
		return ErrInvalidRect
	}
	r, e := w.Rect()
	if e != nil {
		return e
	}
	x, ok := checkedAddInt(bounds.Left, (bounds.Width()-r.Width())/2)
	if !ok {
		return ErrInvalidArgument
	}
	y, ok := checkedAddInt(bounds.Top, (bounds.Height()-r.Height())/2)
	if !ok {
		return ErrInvalidArgument
	}
	return w.Move(Point{x, y})
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
	if bounds.Empty() {
		return ErrInvalidRect
	}
	r, e := w.Rect()
	if e != nil {
		return e
	}
	x, y := r.Left, r.Top
	switch p {
	case SnapCenter:
		var ok bool
		x, ok = checkedAddInt(bounds.Left, (bounds.Width()-r.Width())/2)
		if !ok {
			return ErrInvalidArgument
		}
		y, ok = checkedAddInt(bounds.Top, (bounds.Height()-r.Height())/2)
		if !ok {
			return ErrInvalidArgument
		}
	case SnapLeft:
		x = bounds.Left
	case SnapRight:
		var ok bool
		x, ok = checkedSubInt(bounds.Right, r.Width())
		if !ok {
			return ErrInvalidArgument
		}
	case SnapTop:
		y = bounds.Top
	case SnapBottom:
		var ok bool
		y, ok = checkedSubInt(bounds.Bottom, r.Height())
		if !ok {
			return ErrInvalidArgument
		}
	default:
		return ErrInvalidArgument
	}
	return w.Move(Point{x, y})
}
