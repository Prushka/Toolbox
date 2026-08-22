// Package example contains command-line helpers shared by the automation
// examples. It is internal so it cannot become part of the automation API.
package example

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/Prushka/Toolbox/automation"
)

// QueryFlags exposes the same selectors as automation.WindowQuery.
type QueryFlags struct {
	Title, TitleContains, Class, Process string
	PID                                  uint
	VisibleOnly                          bool
}

func (q *QueryFlags) Bind(fs *flag.FlagSet, visibleDefault bool) {
	fs.StringVar(&q.Title, "title", "", "exact window title (case-insensitive)")
	fs.StringVar(&q.TitleContains, "title-contains", "", "window title substring (case-insensitive)")
	fs.StringVar(&q.Class, "class", "", "exact window class (case-insensitive)")
	fs.StringVar(&q.Process, "process", "", "process basename or full path")
	fs.UintVar(&q.PID, "pid", 0, "process ID")
	fs.BoolVar(&q.VisibleOnly, "visible", visibleDefault, "match only visible windows")
}

func (q QueryFlags) Query() (automation.WindowQuery, error) {
	if uint64(q.PID) > uint64(^uint32(0)) {
		return automation.WindowQuery{}, fmt.Errorf("pid %d exceeds uint32", q.PID)
	}
	return automation.WindowQuery{
		Title:         strings.TrimSpace(q.Title),
		TitleContains: strings.TrimSpace(q.TitleContains),
		Class:         strings.TrimSpace(q.Class),
		Process:       strings.TrimSpace(q.Process),
		PID:           uint32(q.PID),
		VisibleOnly:   q.VisibleOnly,
	}, nil
}

func (q QueryFlags) HasSelector() bool {
	return strings.TrimSpace(q.Title) != "" ||
		strings.TrimSpace(q.TitleContains) != "" ||
		strings.TrimSpace(q.Class) != "" ||
		strings.TrimSpace(q.Process) != "" || q.PID != 0
}

// Resolve returns the first matching window, or the foreground window when no
// selector was supplied.
func (q QueryFlags) Resolve() (automation.Window, error) {
	if !q.HasSelector() {
		return automation.ActiveWindow()
	}
	query, err := q.Query()
	if err != nil {
		return automation.Window{}, err
	}
	return automation.FindWindow(query)
}

func ParseRect(s string) (automation.Rect, error) {
	if strings.TrimSpace(s) == "" {
		return automation.Rect{}, nil
	}
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return automation.Rect{}, fmt.Errorf("rectangle must be left,top,right,bottom")
	}
	values := [4]int{}
	for i, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return automation.Rect{}, fmt.Errorf("invalid rectangle coordinate %q: %w", part, err)
		}
		values[i] = value
	}
	r := automation.Rect{Left: values[0], Top: values[1], Right: values[2], Bottom: values[3]}
	if r.Empty() {
		return automation.Rect{}, automation.ErrInvalidRect
	}
	return r, nil
}

func ParseRGB(s string) (automation.RGB, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return automation.RGB{}, fmt.Errorf("color must be a six-digit RRGGBB value")
	}
	value, err := strconv.ParseUint(s, 16, 24)
	if err != nil {
		return automation.RGB{}, fmt.Errorf("invalid color %q: %w", s, err)
	}
	return automation.RGBFromUint32(uint32(value)), nil
}

func ParseTolerance(s string) (automation.ColorTolerance, error) {
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) != 1 && len(parts) != 3 {
		return automation.ColorTolerance{}, fmt.Errorf("tolerance must be n or r,g,b")
	}
	values := [3]uint8{}
	for i, part := range parts {
		value, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
		if err != nil {
			return automation.ColorTolerance{}, fmt.Errorf("invalid tolerance %q: %w", part, err)
		}
		if len(parts) == 1 {
			values = [3]uint8{uint8(value), uint8(value), uint8(value)}
			break
		}
		values[i] = uint8(value)
	}
	return automation.ColorTolerance{R: values[0], G: values[1], B: values[2]}, nil
}

func ParseCaptureMethod(s string) (automation.CaptureMethod, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "visible":
		return automation.CaptureVisible, nil
	case "print", "printwindow":
		return automation.CapturePrintWindow, nil
	case "auto":
		return automation.CaptureAuto, nil
	default:
		return 0, fmt.Errorf("capture method must be visible, print, or auto")
	}
}

// Strings implements flag.Value for repeatable string flags such as -arg.
type Strings []string

func (s *Strings) String() string { return strings.Join(*s, ", ") }

func (s *Strings) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// ModifierFlags provides the four modifier groups supported by RegisterHotKey.
type ModifierFlags struct {
	Alt, Control, Shift, Win bool
}

func (m *ModifierFlags) Bind(fs *flag.FlagSet) {
	fs.BoolVar(&m.Alt, "alt", false, "require Alt")
	fs.BoolVar(&m.Control, "ctrl", false, "require Ctrl")
	fs.BoolVar(&m.Shift, "shift", false, "require Shift")
	fs.BoolVar(&m.Win, "win", false, "require the Windows key")
}

func (m ModifierFlags) Modifiers() automation.Modifiers {
	var modifiers automation.Modifiers
	if m.Alt {
		modifiers |= automation.ModifierAlt
	}
	if m.Control {
		modifiers |= automation.ModifierControl
	}
	if m.Shift {
		modifiers |= automation.ModifierShift
	}
	if m.Win {
		modifiers |= automation.ModifierWin
	}
	return modifiers
}

func FormatModifiers(modifiers automation.Modifiers) string {
	parts := make([]string, 0, 4)
	if modifiers&automation.ModifierControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if modifiers&automation.ModifierAlt != 0 {
		parts = append(parts, "Alt")
	}
	if modifiers&automation.ModifierShift != 0 {
		parts = append(parts, "Shift")
	}
	if modifiers&automation.ModifierWin != 0 {
		parts = append(parts, "Win")
	}
	return strings.Join(parts, "+")
}

func ParseMouseButton(value string) (automation.MouseButton, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "primary":
		return automation.MousePrimary, nil
	case "secondary":
		return automation.MouseSecondary, nil
	case "middle":
		return automation.MouseMiddle, nil
	case "x1":
		return automation.MouseX1, nil
	case "x2":
		return automation.MouseX2, nil
	default:
		return 0, fmt.Errorf("mouse button must be primary, secondary, middle, x1, or x2")
	}
}

func FormatMouseButton(button automation.MouseButton) string {
	switch button {
	case automation.MousePrimary:
		return "primary"
	case automation.MouseSecondary:
		return "secondary"
	case automation.MouseMiddle:
		return "middle"
	case automation.MouseX1:
		return "x1"
	case automation.MouseX2:
		return "x2"
	default:
		return fmt.Sprintf("button-%d", button)
	}
}

func ParseMouseTrigger(value string) (automation.MouseTrigger, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "press", "down":
		return automation.MousePress, nil
	case "release", "up":
		return automation.MouseRelease, nil
	case "both":
		return automation.MousePressAndRelease, nil
	default:
		return 0, fmt.Errorf("mouse trigger must be press, release, or both")
	}
}
