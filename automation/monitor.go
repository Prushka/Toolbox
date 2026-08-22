package automation

// Modifiers is a set of modifier keys used by keyboard and mouse hotkeys.
// RegisterHotKey cannot distinguish the left and right variants.
type Modifiers uint8

const (
	ModifierAlt Modifiers = 1 << iota
	ModifierControl
	ModifierShift
	ModifierWin
)

const ModifierCtrl = ModifierControl

const validModifiers = ModifierAlt | ModifierControl | ModifierShift | ModifierWin

func (m Modifiers) valid() bool { return m&^validModifiers == 0 }

// BindingID identifies one registration owned by an InputMonitor.
type BindingID uint32

// KeyboardHotkey describes a system-wide keyboard hotkey. Auto-repeat is
// suppressed unless AllowRepeat is true.
type KeyboardHotkey struct {
	Key         Key
	Modifiers   Modifiers
	AllowRepeat bool
}

// MouseTrigger selects which edge of a mouse button produces an event.
type MouseTrigger uint8

const (
	MousePress MouseTrigger = iota
	MouseRelease
	MousePressAndRelease
)

func (t MouseTrigger) valid() bool { return t <= MousePressAndRelease }

// MouseHotkey describes a pass-through mouse-button binding. Modifiers match
// exactly unless AllowExtraModifiers is true. IgnoreInjected filters events
// Windows marks as injected.
type MouseHotkey struct {
	Button              MouseButton
	Modifiers           Modifiers
	Trigger             MouseTrigger
	AllowExtraModifiers bool
	IgnoreInjected      bool
}

func (h MouseHotkey) valid() bool {
	return h.Button >= MousePrimary && h.Button <= MouseX2 && h.Modifiers.valid() && h.Trigger.valid()
}

// InputEventKind identifies the source and edge of a monitored input event.
type InputEventKind uint8

const (
	EventKeyboardHotkey InputEventKind = iota + 1
	EventMousePress
	EventMouseRelease
)

// InputEvent is an immutable event produced by InputMonitor. MessageTime is
// the native millisecond timestamp and wraps with the Windows tick counter.
// Position is populated for mouse events and uses screen coordinates.
type InputEvent struct {
	Binding                BindingID
	Kind                   InputEventKind
	Key                    Key
	Button                 MouseButton
	Modifiers              Modifiers
	Position               Point
	MessageTime            uint32
	Injected               bool
	LowerIntegrityInjected bool
}

// InputMonitorOptions configures event delivery. Buffer zero uses a bounded
// default. The native message thread never waits for a consumer; use
// DroppedEvents to detect overflow.
type InputMonitorOptions struct {
	Buffer int
}

const (
	defaultInputEventBuffer = 64
	maxInputEventBuffer     = 1 << 16
)

func inputEventBufferSize(options InputMonitorOptions) (int, error) {
	if options.Buffer < 0 || options.Buffer > maxInputEventBuffer {
		return 0, ErrInvalidArgument
	}
	if options.Buffer == 0 {
		return defaultInputEventBuffer, nil
	}
	return options.Buffer, nil
}
