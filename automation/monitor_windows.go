//go:build windows

package automation

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procRegisterHotKey      = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey    = user32.NewProc("UnregisterHotKey")
	procGetMessage          = user32.NewProc("GetMessageW")
	procPeekMessage         = user32.NewProc("PeekMessageW")
	procPostThreadMessage   = user32.NewProc("PostThreadMessageW")
	procSetWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetCurrentThreadID  = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentThreadId")
	procGetModuleHandle     = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW")
)

const (
	wmHotkey                = 0x0312
	wmApp                   = 0x8000
	wmInputMonitorCommand   = wmApp + 0x3a1
	pmNoRemove              = 0x0000
	modifierNoRepeat        = 0x4000
	whMouseLowLevel         = 14
	maxMonitorBindingID     = 0xbfff
	maxMouseMonitorBindings = 256
	wmLButtonDown           = 0x0201
	wmLButtonUp             = 0x0202
	wmRButtonDown           = 0x0204
	wmRButtonUp             = 0x0205
	wmMButtonDown           = 0x0207
	wmMButtonUp             = 0x0208
	wmXButtonDown           = 0x020b
	wmXButtonUp             = 0x020c
	xButton1                = 1
	xButton2                = 2
	lowLevelMouseInjected   = 0x00000001
	lowLevelMouseLowerIL    = 0x00000002
)

type winMessage struct {
	HWND    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   winPoint
	Private uint32
}

type lowLevelMouseEvent struct {
	Point     winPoint
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type monitorCommandKind uint8

const (
	monitorRegisterKeyboard monitorCommandKind = iota + 1
	monitorRegisterMouse
	monitorUnregister
	monitorClose
)

type monitorCommand struct {
	kind     monitorCommandKind
	keyboard KeyboardHotkey
	mouse    MouseHotkey
	id       BindingID
	response chan monitorResult
}

type monitorResult struct {
	id  BindingID
	err error
}

type mouseMonitorBinding struct {
	id      BindingID
	binding MouseHotkey
}

type mouseMonitorSnapshot struct {
	bindings []mouseMonitorBinding
}

// InputMonitor owns one Windows message loop for all of a process's monitored
// keyboard and mouse bindings. Create one monitor and share it among callers.
type InputMonitor struct {
	events   chan InputEvent
	done     chan struct{}
	commands chan monitorCommand

	requestMu      sync.Mutex
	closeRequested bool
	threadID       atomic.Uint32
	dropped        atomic.Uint64

	errMu sync.RWMutex
	err   error

	eventMu   sync.RWMutex
	accepting bool

	// The following fields are owned by the locked message-loop goroutine.
	keyboardBindings map[BindingID]KeyboardHotkey
	mouseBindings    map[BindingID]MouseHotkey
	nextID           BindingID
	mouseHook        uintptr
	mouseSnapshot    atomic.Pointer[mouseMonitorSnapshot]
}

var (
	activeInputMonitorMu sync.Mutex
	activeInputMonitor   *InputMonitor
	mouseHookMonitor     atomic.Pointer[InputMonitor]
)

// NewInputMonitor starts the process-wide input monitor. Only one may be
// active at a time; register all required bindings on that monitor.
func NewInputMonitor(options InputMonitorOptions) (*InputMonitor, error) {
	buffer, err := inputEventBufferSize(options)
	if err != nil {
		return nil, err
	}
	m := &InputMonitor{
		events:           make(chan InputEvent, buffer),
		done:             make(chan struct{}),
		commands:         make(chan monitorCommand, 1),
		accepting:        true,
		keyboardBindings: make(map[BindingID]KeyboardHotkey),
		mouseBindings:    make(map[BindingID]MouseHotkey),
	}

	activeInputMonitorMu.Lock()
	if activeInputMonitor != nil {
		activeInputMonitorMu.Unlock()
		return nil, ErrMonitorActive
	}
	activeInputMonitor = m
	activeInputMonitorMu.Unlock()

	ready := make(chan error, 1)
	go m.messageLoop(ready)
	if err := <-ready; err != nil {
		<-m.done
		return nil, err
	}
	return m, nil
}

// RegisterKeyboard registers a global keyboard hotkey through RegisterHotKey.
// The registration is owned by the monitor until explicitly removed or closed.
func (m *InputMonitor) RegisterKeyboard(hotkey KeyboardHotkey) (BindingID, error) {
	if !hotkey.Key.Valid() || isMouseVirtualKey(hotkey.Key) || !hotkey.Modifiers.valid() {
		return 0, ErrInvalidArgument
	}
	result, err := m.request(monitorCommand{kind: monitorRegisterKeyboard, keyboard: hotkey})
	return result.id, err
}

// RegisterMouse registers a pass-through mouse-button binding. The low-level
// mouse hook exists only while at least one mouse binding is registered.
func (m *InputMonitor) RegisterMouse(hotkey MouseHotkey) (BindingID, error) {
	if !hotkey.valid() {
		return 0, ErrInvalidArgument
	}
	result, err := m.request(monitorCommand{kind: monitorRegisterMouse, mouse: hotkey})
	return result.id, err
}

// Unregister removes a keyboard or mouse binding.
func (m *InputMonitor) Unregister(id BindingID) error {
	if id == 0 {
		return ErrInvalidArgument
	}
	_, err := m.request(monitorCommand{kind: monitorUnregister, id: id})
	return err
}

// Events returns the monitor's bounded event stream. The channel closes when
// the monitor stops; call Err to distinguish failure from a normal close.
func (m *InputMonitor) Events() <-chan InputEvent {
	if m == nil {
		return nil
	}
	return m.events
}

// Done closes after native registrations and hooks have been released.
func (m *InputMonitor) Done() <-chan struct{} {
	if m == nil {
		return nil
	}
	return m.done
}

// DroppedEvents reports events discarded because the event buffer was full.
func (m *InputMonitor) DroppedEvents() uint64 {
	if m == nil {
		return 0
	}
	return m.dropped.Load()
}

// Err reports the terminal message-loop or cleanup error, if any.
func (m *InputMonitor) Err() error {
	if m == nil {
		return ErrMonitorClosed
	}
	m.errMu.RLock()
	defer m.errMu.RUnlock()
	return m.err
}

// Close unregisters every hotkey, removes the mouse hook, and stops the native
// message loop. It is idempotent and safe for concurrent callers.
func (m *InputMonitor) Close() error {
	if m == nil {
		return nil
	}
	m.requestMu.Lock()
	if m.closeRequested {
		m.requestMu.Unlock()
		<-m.done
		return m.Err()
	}
	select {
	case <-m.done:
		m.closeRequested = true
		m.requestMu.Unlock()
		return m.Err()
	default:
	}
	m.closeRequested = true
	_, err := m.requestLocked(monitorCommand{kind: monitorClose})
	if err != nil {
		select {
		case <-m.done:
		default:
			m.closeRequested = false
		}
		m.requestMu.Unlock()
		return err
	}
	m.requestMu.Unlock()
	<-m.done
	return m.Err()
}

func (m *InputMonitor) request(command monitorCommand) (monitorResult, error) {
	if m == nil {
		return monitorResult{}, ErrMonitorClosed
	}
	m.requestMu.Lock()
	defer m.requestMu.Unlock()
	if m.closeRequested {
		return monitorResult{}, ErrMonitorClosed
	}
	return m.requestLocked(command)
}

func (m *InputMonitor) requestLocked(command monitorCommand) (monitorResult, error) {
	select {
	case <-m.done:
		return monitorResult{}, m.stoppedError()
	default:
	}
	command.response = make(chan monitorResult, 1)
	select {
	case m.commands <- command:
	case <-m.done:
		return monitorResult{}, m.stoppedError()
	}
	threadID := m.threadID.Load()
	if threadID == 0 {
		<-m.commands
		return monitorResult{}, ErrMonitorClosed
	}
	if ret, _, callErr := procPostThreadMessage.Call(uintptr(threadID), wmInputMonitorCommand, 0, 0); ret == 0 {
		select {
		case <-m.commands:
		default:
		}
		return monitorResult{}, winCallError(callErr, "PostThreadMessage(input monitor) failed")
	}
	select {
	case result := <-command.response:
		return result, result.err
	case <-m.done:
		return monitorResult{}, m.stoppedError()
	}
}

func (m *InputMonitor) stoppedError() error {
	if err := m.Err(); err != nil {
		return err
	}
	return ErrMonitorClosed
}

func (m *InputMonitor) messageLoop(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var loopErr error
	defer func() {
		cleanupErr := m.cleanupNative()
		m.finish(errors.Join(loopErr, cleanupErr))
	}()
	for _, proc := range []*windows.LazyProc{
		procRegisterHotKey, procUnregisterHotKey, procGetMessage, procPeekMessage,
		procPostThreadMessage, procGetCurrentThreadID,
	} {
		if err := proc.Find(); err != nil {
			loopErr = err
			ready <- err
			return
		}
	}

	threadID, _, callErr := procGetCurrentThreadID.Call()
	if threadID == 0 {
		loopErr = winCallError(callErr, "GetCurrentThreadId failed")
		ready <- loopErr
		return
	}
	var message winMessage
	procPeekMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0, pmNoRemove)
	m.threadID.Store(uint32(threadID))
	ready <- nil

	for {
		ret, _, callErr := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			loopErr = winCallError(callErr, "GetMessage(input monitor) failed")
			return
		case 0:
			return
		}
		switch message.Message {
		case wmHotkey:
			m.handleKeyboardEvent(BindingID(message.WParam), message.Time)
		case wmInputMonitorCommand:
			select {
			case command := <-m.commands:
				if m.handleCommand(command) {
					return
				}
			default:
			}
		}
	}
}

func (m *InputMonitor) handleCommand(command monitorCommand) bool {
	var result monitorResult
	switch command.kind {
	case monitorRegisterKeyboard:
		result.id, result.err = m.registerKeyboardNative(command.keyboard)
	case monitorRegisterMouse:
		result.id, result.err = m.registerMouseNative(command.mouse)
	case monitorUnregister:
		result.err = m.unregisterNative(command.id)
	case monitorClose:
		command.response <- result
		return true
	default:
		result.err = ErrInvalidArgument
	}
	command.response <- result
	return false
}

func (m *InputMonitor) registerKeyboardNative(hotkey KeyboardHotkey) (BindingID, error) {
	id, err := m.allocateBindingID()
	if err != nil {
		return 0, err
	}
	modifiers := uintptr(hotkey.Modifiers)
	if !hotkey.AllowRepeat {
		modifiers |= modifierNoRepeat
	}
	if ret, _, callErr := procRegisterHotKey.Call(0, uintptr(id), modifiers, uintptr(hotkey.Key)); ret == 0 {
		return 0, winCallError(callErr, fmt.Sprintf("RegisterHotKey(%s) failed", hotkey.Key))
	}
	m.keyboardBindings[id] = hotkey
	return id, nil
}

func (m *InputMonitor) registerMouseNative(hotkey MouseHotkey) (BindingID, error) {
	if len(m.mouseBindings) >= maxMouseMonitorBindings {
		return 0, fmt.Errorf("%w: mouse binding limit is %d", ErrInvalidArgument, maxMouseMonitorBindings)
	}
	id, err := m.allocateBindingID()
	if err != nil {
		return 0, err
	}
	if m.mouseHook == 0 {
		if err := m.installMouseHook(); err != nil {
			return 0, err
		}
	}
	m.mouseBindings[id] = hotkey
	m.rebuildMouseSnapshot()
	return id, nil
}

func (m *InputMonitor) unregisterNative(id BindingID) error {
	if _, ok := m.keyboardBindings[id]; ok {
		if ret, _, callErr := procUnregisterHotKey.Call(0, uintptr(id)); ret == 0 {
			return winCallError(callErr, "UnregisterHotKey failed")
		}
		delete(m.keyboardBindings, id)
		return nil
	}
	if _, ok := m.mouseBindings[id]; !ok {
		return ErrNotFound
	}
	delete(m.mouseBindings, id)
	m.rebuildMouseSnapshot()
	if len(m.mouseBindings) == 0 {
		return m.removeMouseHook()
	}
	return nil
}

func (m *InputMonitor) allocateBindingID() (BindingID, error) {
	for attempts := 0; attempts < maxMonitorBindingID; attempts++ {
		m.nextID++
		if m.nextID == 0 || m.nextID > maxMonitorBindingID {
			m.nextID = 1
		}
		if _, exists := m.keyboardBindings[m.nextID]; exists {
			continue
		}
		if _, exists := m.mouseBindings[m.nextID]; exists {
			continue
		}
		return m.nextID, nil
	}
	return 0, fmt.Errorf("%w: input monitor binding IDs exhausted", ErrInvalidArgument)
}

func (m *InputMonitor) installMouseHook() error {
	for _, proc := range []*windows.LazyProc{
		procSetWindowsHookEx, procUnhookWindowsHookEx, procCallNextHookEx,
		procGetModuleHandle, procGetAsyncKeyState,
	} {
		if err := proc.Find(); err != nil {
			return err
		}
	}
	module, _, callErr := procGetModuleHandle.Call(0)
	if module == 0 {
		return winCallError(callErr, "GetModuleHandle failed")
	}
	if !mouseHookMonitor.CompareAndSwap(nil, m) {
		return ErrMonitorActive
	}
	hook, _, callErr := procSetWindowsHookEx.Call(whMouseLowLevel, mouseHookCallback, module, 0)
	if hook == 0 {
		mouseHookMonitor.CompareAndSwap(m, nil)
		return winCallError(callErr, "SetWindowsHookEx(WH_MOUSE_LL) failed")
	}
	m.mouseHook = hook
	return nil
}

func (m *InputMonitor) removeMouseHook() error {
	if m.mouseHook == 0 {
		mouseHookMonitor.CompareAndSwap(m, nil)
		return nil
	}
	if ret, _, callErr := procUnhookWindowsHookEx.Call(m.mouseHook); ret == 0 {
		return winCallError(callErr, "UnhookWindowsHookEx failed")
	}
	m.mouseHook = 0
	mouseHookMonitor.CompareAndSwap(m, nil)
	return nil
}

func (m *InputMonitor) rebuildMouseSnapshot() {
	if len(m.mouseBindings) == 0 {
		m.mouseSnapshot.Store(nil)
		return
	}
	bindings := make([]mouseMonitorBinding, 0, len(m.mouseBindings))
	for id, binding := range m.mouseBindings {
		bindings = append(bindings, mouseMonitorBinding{id: id, binding: binding})
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].id < bindings[j].id })
	m.mouseSnapshot.Store(&mouseMonitorSnapshot{bindings: bindings})
}

func (m *InputMonitor) handleKeyboardEvent(id BindingID, messageTime uint32) {
	hotkey, ok := m.keyboardBindings[id]
	if !ok {
		return
	}
	m.publish(InputEvent{
		Binding:     id,
		Kind:        EventKeyboardHotkey,
		Key:         hotkey.Key,
		Modifiers:   hotkey.Modifiers,
		MessageTime: messageTime,
	})
}

func (m *InputMonitor) dispatchMouseEvent(message uint32, native *lowLevelMouseEvent, modifiers Modifiers) {
	if native == nil {
		return
	}
	button, pressed, ok := mouseMessageButton(message, native.MouseData)
	if !ok {
		return
	}
	snapshot := m.mouseSnapshot.Load()
	if snapshot == nil {
		return
	}
	injected := native.Flags&lowLevelMouseInjected != 0
	for _, entry := range snapshot.bindings {
		binding := entry.binding
		if binding.Button != button || binding.IgnoreInjected && injected || !mouseTriggerMatches(binding.Trigger, pressed) {
			continue
		}
		if binding.AllowExtraModifiers {
			if modifiers&binding.Modifiers != binding.Modifiers {
				continue
			}
		} else if modifiers != binding.Modifiers {
			continue
		}
		kind := EventMouseRelease
		if pressed {
			kind = EventMousePress
		}
		m.publish(InputEvent{
			Binding:                entry.id,
			Kind:                   kind,
			Button:                 button,
			Modifiers:              modifiers,
			Position:               Point{X: int(native.Point.X), Y: int(native.Point.Y)},
			MessageTime:            native.Time,
			Injected:               injected,
			LowerIntegrityInjected: native.Flags&lowLevelMouseLowerIL != 0,
		})
	}
}

func (m *InputMonitor) publish(event InputEvent) {
	m.eventMu.RLock()
	defer m.eventMu.RUnlock()
	if !m.accepting {
		return
	}
	select {
	case m.events <- event:
	default:
		m.dropped.Add(1)
	}
}

func (m *InputMonitor) cleanupNative() error {
	var cleanupErrors []error
	for id := range m.keyboardBindings {
		if ret, _, callErr := procUnregisterHotKey.Call(0, uintptr(id)); ret == 0 {
			cleanupErrors = append(cleanupErrors, winCallError(callErr, "UnregisterHotKey during close failed"))
		}
		delete(m.keyboardBindings, id)
	}
	clear(m.mouseBindings)
	m.mouseSnapshot.Store(nil)
	if err := m.removeMouseHook(); err != nil {
		cleanupErrors = append(cleanupErrors, err)
		m.mouseHook = 0
		mouseHookMonitor.CompareAndSwap(m, nil)
	}
	return errors.Join(cleanupErrors...)
}

func (m *InputMonitor) finish(err error) {
	m.threadID.Store(0)
	m.errMu.Lock()
	m.err = err
	m.errMu.Unlock()

	activeInputMonitorMu.Lock()
	if activeInputMonitor == m {
		activeInputMonitor = nil
	}
	activeInputMonitorMu.Unlock()

	m.eventMu.Lock()
	m.accepting = false
	close(m.events)
	m.eventMu.Unlock()
	close(m.done)
}

func isMouseVirtualKey(key Key) bool {
	switch key {
	case KeyLButton, KeyRButton, KeyMButton, KeyXButton1, KeyXButton2:
		return true
	default:
		return false
	}
}

func mouseTriggerMatches(trigger MouseTrigger, pressed bool) bool {
	return trigger == MousePressAndRelease || trigger == MousePress && pressed || trigger == MouseRelease && !pressed
}

func mouseMessageButton(message, mouseData uint32) (MouseButton, bool, bool) {
	switch message {
	case wmLButtonDown:
		return MousePrimary, true, true
	case wmLButtonUp:
		return MousePrimary, false, true
	case wmRButtonDown:
		return MouseSecondary, true, true
	case wmRButtonUp:
		return MouseSecondary, false, true
	case wmMButtonDown:
		return MouseMiddle, true, true
	case wmMButtonUp:
		return MouseMiddle, false, true
	case wmXButtonDown, wmXButtonUp:
		button := uint16(mouseData >> 16)
		if button == xButton1 {
			return MouseX1, message == wmXButtonDown, true
		}
		if button == xButton2 {
			return MouseX2, message == wmXButtonDown, true
		}
	}
	return 0, false, false
}

func currentMonitorModifiers() Modifiers {
	var modifiers Modifiers
	if asyncKeyDown(KeyAlt) {
		modifiers |= ModifierAlt
	}
	if asyncKeyDown(KeyCtrl) {
		modifiers |= ModifierControl
	}
	if asyncKeyDown(KeyShift) {
		modifiers |= ModifierShift
	}
	if asyncKeyDown(KeyLWin) || asyncKeyDown(KeyRWin) {
		modifiers |= ModifierWin
	}
	return modifiers
}

var mouseHookCallback = windows.NewCallback(func(code, message uintptr, data unsafe.Pointer) uintptr {
	if int32(code) >= 0 && data != nil {
		if monitor := mouseHookMonitor.Load(); monitor != nil {
			monitor.dispatchMouseEvent(uint32(message), (*lowLevelMouseEvent)(data), currentMonitorModifiers())
		}
	}
	result, _, _ := syscall.SyscallN(procCallNextHookEx.Addr(), 0, code, message, uintptr(data))
	return result
})
