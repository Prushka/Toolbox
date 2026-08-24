package hid

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultTimeout  = 2 * time.Second
	defaultTapDelay = 20 * time.Millisecond
	cleanupTimeout  = 250 * time.Millisecond
)

var (
	ErrClosed             = errors.New("hid: client is closed")
	ErrUnsupported        = errors.New("hid: operation is not supported on this platform")
	ErrWindowPointerMoved = errors.New("hid: prepared window pointer moved")
)

// Capability describes a firmware feature.
type Capability uint16

const (
	CapabilityKeyboard Capability = 1 << iota
	CapabilityRelativeMouse
	CapabilityAbsoluteMouse
	CapabilityHorizontalWheel
	CapabilityUSBDetach
)

// Info is returned by the board during connection negotiation.
type Info struct {
	FirmwareMajor   byte
	FirmwareMinor   byte
	ProtocolVersion byte
	Capabilities    Capability
	MaximumPayload  byte
	WatchdogTimeout time.Duration
}

// Button is a mouse button bit. Values can be combined.
type Button byte

const (
	ButtonLeft    Button = 1 << 0
	ButtonRight   Button = 1 << 1
	ButtonMiddle  Button = 1 << 2
	ButtonBack    Button = 1 << 3
	ButtonForward Button = 1 << 4
)

type config struct {
	timeout  time.Duration
	tapDelay time.Duration
}

// Option configures a Client.
type Option func(*config) error

// WithTimeout sets the maximum time to wait for a firmware acknowledgement.
func WithTimeout(timeout time.Duration) Option {
	return func(configuration *config) error {
		if timeout <= 0 {
			return errors.New("hid: timeout must be positive")
		}
		configuration.timeout = timeout
		return nil
	}
}

// WithTapDelay sets how long Press and Click keep inputs down.
func WithTapDelay(delay time.Duration) Option {
	return func(configuration *config) error {
		if delay < 0 {
			return errors.New("hid: tap delay cannot be negative")
		}
		configuration.tapDelay = delay
		return nil
	}
}

// DeviceError is a negative acknowledgement from the firmware.
type DeviceError struct {
	Status byte
}

func (err *DeviceError) Error() string {
	message := "unknown device error"
	switch status(err.Status) {
	case statusUnknownOp:
		message = "unknown command"
	case statusBadPayload:
		message = "invalid command payload"
	case statusHIDFailure:
		message = "HID report could not be sent"
	case statusBadChecksum:
		message = "request checksum failed"
	case statusBadVersion:
		message = "protocol version mismatch"
	}
	return fmt.Sprintf("hid: firmware rejected command: %s (status %d)", message, err.Status)
}

type responseResult struct {
	frame wireFrame
	err   error
}

type windowPointerState struct {
	valid            bool
	screenX, screenY int
	originX, originY int
	clientX, clientY int
}

// Client is safe for concurrent use. Commands are serialized because the
// firmware processes one acknowledged request at a time.
type Client struct {
	transport io.ReadWriteCloser
	reader    *bufio.Reader
	timeout   time.Duration
	tapDelay  time.Duration

	commandMu       chan struct{}
	sequence        byte
	responses       chan responseResult
	readerDone      chan struct{}
	readerOnce      sync.Once
	readErrMu       sync.RWMutex
	readErr         error
	closed          atomic.Bool
	closeOnce       sync.Once
	closeErr        error
	infoMu          sync.RWMutex
	info            Info
	infoKnown       bool
	windowPointerMu sync.Mutex
	windowPointer   windowPointerState
}

// NewClient binds a client to an already-open byte stream. Most callers should
// use Open, while tests and custom transports can use this constructor.
func NewClient(transport io.ReadWriteCloser, options ...Option) (*Client, error) {
	if transport == nil {
		return nil, errors.New("hid: transport is nil")
	}
	configuration := config{timeout: defaultTimeout, tapDelay: defaultTapDelay}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&configuration); err != nil {
			return nil, err
		}
	}

	client := &Client{
		transport:  transport,
		reader:     bufio.NewReader(transport),
		timeout:    configuration.timeout,
		tapDelay:   configuration.tapDelay,
		commandMu:  make(chan struct{}, 1),
		responses:  make(chan responseResult, 1),
		readerDone: make(chan struct{}),
	}
	client.commandMu <- struct{}{}
	go client.readResponses()
	return client, nil
}

func (client *Client) readResponses() {
	defer client.signalReaderDone()
	for {
		frame, err := readFrame(client.reader, responseMagic)
		if err != nil {
			if isRecoverableFrameError(err) {
				select {
				case client.responses <- responseResult{frame: frame, err: err}:
				case <-client.readerDone:
					return
				}
				continue
			}
			if !client.closed.Load() {
				client.setReaderError(err)
			}
			return
		}
		select {
		case client.responses <- responseResult{frame: frame}:
		case <-client.readerDone:
			return
		}
	}
}

func isRecoverableFrameError(err error) bool {
	return errors.Is(err, errBadChecksum) || errors.Is(err, errBadVersion) ||
		errors.Is(err, errFrameLarge)
}

func (client *Client) setReaderError(err error) {
	client.readErrMu.Lock()
	if client.readErr == nil {
		client.readErr = err
	}
	client.readErrMu.Unlock()
}

func (client *Client) readerError() error {
	client.readErrMu.RLock()
	defer client.readErrMu.RUnlock()
	return client.readErr
}

func (client *Client) signalReaderDone() {
	client.readerOnce.Do(func() { close(client.readerDone) })
}

func (client *Client) acquireCommand(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-client.readerDone:
		if client.closed.Load() {
			return ErrClosed
		}
		if err := client.readerError(); err != nil {
			return fmt.Errorf("hid: read response: %w", err)
		}
		return ErrClosed
	case <-client.commandMu:
		if err := ctx.Err(); err != nil {
			client.releaseCommand()
			return err
		}
		return nil
	}
}

func (client *Client) releaseCommand() {
	client.commandMu <- struct{}{}
}

func (client *Client) drainResponses() {
	for {
		select {
		case <-client.responses:
		default:
			return
		}
	}
}

func (client *Client) transact(ctx context.Context, operation opcode, payload []byte) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("hid: context is nil")
	}
	if err := client.acquireCommand(ctx); err != nil {
		return nil, err
	}
	defer client.releaseCommand()

	if client.closed.Load() {
		return nil, ErrClosed
	}
	if err := client.readerError(); err != nil {
		return nil, fmt.Errorf("hid: read response: %w", err)
	}
	client.drainResponses()
	client.sequence++
	sequence := client.sequence
	request, err := encodeFrame(requestMagic, sequence, byte(operation), payload)
	if err != nil {
		return nil, err
	}
	if err := writeAll(client.transport, request); err != nil {
		return nil, fmt.Errorf("hid: write command: %w", err)
	}

	timer := time.NewTimer(client.timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, fmt.Errorf("hid: command 0x%02X timed out after %s", operation, client.timeout)
		case result := <-client.responses:
			if result.frame.sequence != sequence {
				continue
			}
			if result.err != nil {
				return nil, fmt.Errorf("hid: read response: %w", result.err)
			}
			if status(result.frame.code) != statusOK {
				return nil, &DeviceError{Status: result.frame.code}
			}
			return result.frame.payload, nil
		case <-client.readerDone:
			// A device may close immediately after writing its final ACK. Give
			// an already-buffered response priority over the terminal signal.
			select {
			case result := <-client.responses:
				if result.frame.sequence != sequence {
					continue
				}
				if result.err != nil {
					return nil, fmt.Errorf("hid: read response: %w", result.err)
				}
				if status(result.frame.code) != statusOK {
					return nil, &DeviceError{Status: result.frame.code}
				}
				return result.frame.payload, nil
			default:
			}
			if client.closed.Load() {
				return nil, ErrClosed
			}
			if err := client.readerError(); err != nil {
				return nil, fmt.Errorf("hid: read response: %w", err)
			}
			return nil, ErrClosed
		}
	}
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

// Ping checks that the command channel and firmware are responsive.
func (client *Client) Ping(ctx context.Context) error {
	_, err := client.transact(ctx, opPing, nil)
	return err
}

// Info negotiates the protocol and returns the firmware's capabilities.
func (client *Client) Info(ctx context.Context) (Info, error) {
	payload, err := client.transact(ctx, opInfo, nil)
	if err != nil {
		return Info{}, err
	}
	if len(payload) != 7 {
		return Info{}, fmt.Errorf("hid: malformed info response: got %d bytes", len(payload))
	}
	info := Info{
		FirmwareMajor:   payload[0],
		FirmwareMinor:   payload[1],
		ProtocolVersion: payload[2],
		Capabilities:    Capability(binary.LittleEndian.Uint16(payload[3:5])),
		MaximumPayload:  payload[5],
		WatchdogTimeout: time.Duration(payload[6]) * time.Second,
	}
	if info.ProtocolVersion != protocolVersion {
		return Info{}, fmt.Errorf("hid: firmware protocol %d does not match client protocol %d", info.ProtocolVersion, protocolVersion)
	}
	client.infoMu.Lock()
	client.info = info
	client.infoKnown = true
	client.infoMu.Unlock()
	return info, nil
}

func (client *Client) supports(ctx context.Context, capability Capability) (bool, error) {
	client.infoMu.RLock()
	info, known := client.info, client.infoKnown
	client.infoMu.RUnlock()
	if !known {
		var err error
		info, err = client.Info(ctx)
		if err != nil {
			return false, err
		}
	}
	return info.Capabilities&capability != 0, nil
}

// KeyDown holds one or more keys. A standard boot keyboard report can hold at
// most six non-modifier keys at once; modifiers do not count toward that limit.
func (client *Client) KeyDown(ctx context.Context, keys ...Key) error {
	return client.keys(ctx, opKeyDown, keys)
}

// KeyUp releases one or more keys.
func (client *Client) KeyUp(ctx context.Context, keys ...Key) error {
	return client.keys(ctx, opKeyUp, keys)
}

func (client *Client) keys(ctx context.Context, operation opcode, keys []Key) error {
	if len(keys) == 0 {
		return errors.New("hid: at least one key is required")
	}
	if len(keys) > maxPayload {
		return errFrameLarge
	}
	if err := validateKeys(keys); err != nil {
		return err
	}
	payload := make([]byte, len(keys))
	for index, key := range keys {
		payload[index] = byte(key)
	}
	_, err := client.transact(ctx, operation, payload)
	return err
}

func validateKeys(keys []Key) error {
	nonModifiers := 0
	seen := make(map[Key]struct{}, len(keys))
	for _, key := range keys {
		if !isSupportedKey(key) {
			return fmt.Errorf("hid: unsupported Arduino key value 0x%02X", byte(key))
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("hid: duplicate key 0x%02X", byte(key))
		}
		seen[key] = struct{}{}
		if !isModifier(key) {
			nonModifiers++
		}
	}
	if nonModifiers > 6 {
		return errors.New("hid: a USB boot keyboard supports at most six non-modifier keys at once")
	}
	return nil
}

func isSupportedKey(key Key) bool {
	return key >= 0x20 && key <= 0x7E ||
		key >= KeyLeftCtrl && key <= KeyRightGUI ||
		key >= KeyEnter && key <= KeyTab ||
		key >= KeyCapsLock && key <= KeyKeypadDecimal ||
		key == KeyMenu || key >= KeyF13 && key <= KeyF24
}

func isModifier(key Key) bool {
	return key >= KeyLeftCtrl && key <= KeyRightGUI
}

// Press presses a key or chord, waits for the configured tap delay, and then
// releases the keys in reverse order.
func (client *Client) Press(ctx context.Context, keys ...Key) (err error) {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	if err := validateKeys(keys); err != nil {
		return err
	}
	if err = client.KeyDown(ctx, keys...); err != nil {
		cleanupErr := client.releaseKeyboardBestEffort(ctx)
		return errors.Join(err, cleanupErr)
	}
	defer func() {
		releaseErr := client.keyUpBestEffort(ctx, reverseKeys(keys)...)
		err = errors.Join(err, releaseErr)
	}()
	return waitContext(ctx, client.tapDelay)
}

func reverseKeys(keys []Key) []Key {
	reversed := append([]Key(nil), keys...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed
}

// Type writes US-ASCII text using the firmware's keyboard layout. Newline,
// tab, and backspace are sent as explicit key taps. USB HID has no portable
// Unicode text primitive, so non-ASCII text is rejected.
func (client *Client) Type(ctx context.Context, text string) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	runes := []rune(text)
	for index := 0; index < len(runes); index++ {
		r := runes[index]
		if r == '\r' && index+1 < len(runes) && runes[index+1] == '\n' {
			index++
			continue
		}
		if r == '\n' || r == '\r' || r == '\t' || r == '\b' {
			continue
		}
		if r < 0x20 || r > 0x7E {
			return fmt.Errorf("hid: cannot type %q: USB HID text is limited to US-ASCII", r)
		}
	}

	chunk := make([]byte, 0, maxPayload)
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		_, err := client.transact(ctx, opTypeASCII, chunk)
		chunk = chunk[:0]
		return err
	}

	for index := 0; index < len(runes); index++ {
		r := runes[index]
		if r == '\r' && index+1 < len(runes) && runes[index+1] == '\n' {
			index++
		}
		var special Key
		switch r {
		case '\n', '\r':
			special = KeyEnter
		case '\t':
			special = KeyTab
		case '\b':
			special = KeyBackspace
		default:
			chunk = append(chunk, byte(r))
			if len(chunk) == maxPayload {
				if err := flush(); err != nil {
					return err
				}
			}
			continue
		}

		if err := flush(); err != nil {
			return err
		}
		if err := client.Press(ctx, special); err != nil {
			return err
		}
	}
	return flush()
}

// ReleaseKeyboard releases every keyboard key known to the firmware.
func (client *Client) ReleaseKeyboard(ctx context.Context) error {
	_, err := client.transact(ctx, opKeyboardReset, nil)
	return err
}

// Move moves the hardware pointer by relative HID units. Large movements are
// split into valid signed 8-bit HID reports.
func (client *Client) Move(ctx context.Context, dx, dy int) error {
	client.invalidateWindowPointer()
	return client.moveRelativeReports(ctx, dx, dy)
}

// MoveLinear moves by relative HID counts using reports no larger than four
// counts per axis. This avoids the accelerated response that some raw-input
// applications apply to larger reports.
func (client *Client) MoveLinear(ctx context.Context, dx, dy int) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	client.invalidateWindowPointer()
	return client.moveLinearReports(ctx, dx, dy)
}

func (client *Client) moveLinearReports(ctx context.Context, dx, dy int) error {
	for dx != 0 || dy != 0 {
		x := clampLinearDelta(dx)
		y := clampLinearDelta(dy)
		if err := client.moveRelativeReports(ctx, x, y); err != nil {
			return err
		}
		dx -= x
		dy -= y
	}
	return nil
}

func clampLinearDelta(value int) int {
	if value > 4 {
		return 4
	}
	if value < -4 {
		return -4
	}
	return value
}

func (client *Client) moveRelativeReports(ctx context.Context, dx, dy int) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	for dx != 0 || dy != 0 {
		x := clampDelta(dx)
		y := clampDelta(dy)
		payload := []byte{byte(int8(x)), byte(int8(y)), 0, 0}
		if _, err := client.transact(ctx, opMouseMove, payload); err != nil {
			return err
		}
		dx -= x
		dy -= y
	}
	return nil
}

// MoveAbsolute moves immediately to normalized HID coordinates in the range
// 0..32767. MoveTo is usually more convenient on Windows.
func (client *Client) MoveAbsolute(ctx context.Context, x, y uint16) error {
	client.invalidateWindowPointer()
	return client.moveAbsoluteReport(ctx, x, y)
}

func (client *Client) moveAbsoluteReport(ctx context.Context, x, y uint16) error {
	if x > 32767 || y > 32767 {
		return errors.New("hid: absolute coordinates must be between 0 and 32767")
	}
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[0:2], x)
	binary.LittleEndian.PutUint16(payload[2:4], y)
	_, err := client.transact(ctx, opMouseAbs, payload)
	return err
}

func (client *Client) invalidateWindowPointer() {
	if client == nil {
		return
	}
	client.windowPointerMu.Lock()
	client.windowPointer.valid = false
	client.windowPointerMu.Unlock()
}

// ResetWindowPointer discards cached raw-input pointer calibration. Call this
// after the target application replaces or reloads its input surface; the
// Windows cursor may be unchanged while the application's internal pointer has
// reset.
func (client *Client) ResetWindowPointer() {
	client.invalidateWindowPointer()
}

func (client *Client) noteWindowCursorPosition(x, y int) {
	if client == nil {
		return
	}
	client.windowPointerMu.Lock()
	if client.windowPointer.valid {
		client.windowPointer.screenX = x
		client.windowPointer.screenY = y
	}
	client.windowPointerMu.Unlock()
}

// Scroll scrolls vertically and horizontally. Positive vertical values scroll
// up; positive horizontal values scroll right.
func (client *Client) Scroll(ctx context.Context, vertical, horizontal int) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	for vertical != 0 || horizontal != 0 {
		wheel := clampDelta(vertical)
		pan := clampDelta(horizontal)
		payload := []byte{0, 0, byte(int8(wheel)), byte(int8(pan))}
		if _, err := client.transact(ctx, opMouseMove, payload); err != nil {
			return err
		}
		vertical -= wheel
		horizontal -= pan
	}
	return nil
}

func clampDelta(value int) int {
	if value > 127 {
		return 127
	}
	if value < -127 {
		return -127
	}
	return value
}

// ButtonDown holds one or more mouse buttons.
func (client *Client) ButtonDown(ctx context.Context, buttons ...Button) error {
	return client.mouseButtons(ctx, opMouseDown, buttons)
}

// ButtonUp releases one or more mouse buttons.
func (client *Client) ButtonUp(ctx context.Context, buttons ...Button) error {
	return client.mouseButtons(ctx, opMouseUp, buttons)
}

func (client *Client) mouseButtons(ctx context.Context, operation opcode, buttons []Button) error {
	mask, err := buttonMask(buttons)
	if err != nil {
		return err
	}
	_, err = client.transact(ctx, operation, []byte{byte(mask)})
	return err
}

func buttonMask(buttons []Button) (Button, error) {
	if len(buttons) == 0 {
		return 0, errors.New("hid: at least one mouse button is required")
	}
	var mask Button
	for _, button := range buttons {
		if button == 0 || button & ^Button(0x1F) != 0 {
			return 0, fmt.Errorf("hid: invalid mouse button mask 0x%02X", byte(button))
		}
		mask |= button
	}
	return mask, nil
}

// Click clicks one or more buttons simultaneously.
func (client *Client) Click(ctx context.Context, buttons ...Button) (err error) {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	if _, err := buttonMask(buttons); err != nil {
		return err
	}
	if err = client.ButtonDown(ctx, buttons...); err != nil {
		cleanupErr := client.releaseMouseBestEffort(ctx)
		return errors.Join(err, cleanupErr)
	}
	defer func() {
		releaseErr := client.buttonUpBestEffort(ctx, buttons...)
		err = errors.Join(err, releaseErr)
	}()
	return waitContext(ctx, client.tapDelay)
}

// DoubleClick performs two left-button clicks using the supplied interval.
func (client *Client) DoubleClick(ctx context.Context, interval time.Duration) error {
	if interval < 0 {
		return errors.New("hid: double-click interval cannot be negative")
	}
	if err := client.Click(ctx, ButtonLeft); err != nil {
		return err
	}
	if err := waitContext(ctx, interval); err != nil {
		return err
	}
	return client.Click(ctx, ButtonLeft)
}

// ReleaseMouse releases every mouse button known to the firmware.
func (client *Client) ReleaseMouse(ctx context.Context) error {
	_, err := client.transact(ctx, opMouseReset, nil)
	return err
}

// ReleaseAll releases all keyboard keys and mouse buttons.
func (client *Client) ReleaseAll(ctx context.Context) error {
	_, err := client.transact(ctx, opReleaseAll, nil)
	return err
}

// CycleUSB releases all input, acknowledges the request, detaches the USB
// device for the requested interval, and reattaches it. The Client is closed;
// wait for Windows to enumerate the COM port and call Open again.
func (client *Client) CycleUSB(ctx context.Context, detachedFor time.Duration) error {
	if detachedFor < 250*time.Millisecond || detachedFor > 30*time.Second {
		return errors.New("hid: USB detach duration must be between 250ms and 30s")
	}
	milliseconds := detachedFor.Milliseconds()
	payload := make([]byte, 2)
	binary.LittleEndian.PutUint16(payload, uint16(milliseconds))
	if _, err := client.transact(ctx, opCycleUSB, payload); err != nil {
		return err
	}
	return client.shutdown()
}

// Close makes a best-effort ReleaseAll request and closes the command port.
// Physically unplugging the board is also safe because Windows drops its HID
// state and the firmware watchdog releases locally held state.
func (client *Client) Close() error {
	if !client.closed.Load() {
		ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
		_, _ = client.transact(ctx, opReleaseAll, nil)
		cancel()
	}
	return client.shutdown()
}

func (client *Client) shutdown() error {
	client.closeOnce.Do(func() {
		client.closed.Store(true)
		client.signalReaderDone()
		client.closeErr = client.transport.Close()
	})
	return client.closeErr
}

func (client *Client) cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, cleanupTimeout)
}

func (client *Client) releaseKeyboardBestEffort(ctx context.Context) error {
	cleanupCtx, cancel := client.cleanupContext(ctx)
	defer cancel()
	return client.ReleaseKeyboard(cleanupCtx)
}

func (client *Client) keyUpBestEffort(ctx context.Context, keys ...Key) error {
	cleanupCtx, cancel := client.cleanupContext(ctx)
	defer cancel()
	return client.KeyUp(cleanupCtx, keys...)
}

func (client *Client) releaseMouseBestEffort(ctx context.Context) error {
	cleanupCtx, cancel := client.cleanupContext(ctx)
	defer cancel()
	return client.ReleaseMouse(cleanupCtx)
}

func (client *Client) releaseAllBestEffort(ctx context.Context) error {
	cleanupCtx, cancel := client.cleanupContext(ctx)
	defer cancel()
	return client.ReleaseAll(cleanupCtx)
}

func (client *Client) buttonUpBestEffort(ctx context.Context, buttons ...Button) error {
	cleanupCtx, cancel := client.cleanupContext(ctx)
	defer cancel()
	return client.ButtonUp(cleanupCtx, buttons...)
}

func waitContext(ctx context.Context, delay time.Duration) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	if delay == 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
