package emulation

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
)

var (
	ErrClosed      = errors.New("emulation: client is closed")
	ErrUnsupported = errors.New("emulation: operation is not supported on this platform")
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
			return errors.New("emulation: timeout must be positive")
		}
		configuration.timeout = timeout
		return nil
	}
}

// WithTapDelay sets how long Press and Click keep inputs down.
func WithTapDelay(delay time.Duration) Option {
	return func(configuration *config) error {
		if delay < 0 {
			return errors.New("emulation: tap delay cannot be negative")
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
	return fmt.Sprintf("emulation: firmware rejected command: %s (status %d)", message, err.Status)
}

type responseResult struct {
	frame wireFrame
	err   error
}

// Client is safe for concurrent use. Commands are serialized because the
// firmware processes one acknowledged request at a time.
type Client struct {
	transport io.ReadWriteCloser
	reader    *bufio.Reader
	timeout   time.Duration
	tapDelay  time.Duration

	commandMu sync.Mutex
	sequence  byte
	responses chan responseResult
	done      chan struct{}
	closed    atomic.Bool
	closeOnce sync.Once
}

// NewClient binds a client to an already-open byte stream. Most callers should
// use Open, while tests and custom transports can use this constructor.
func NewClient(transport io.ReadWriteCloser, options ...Option) (*Client, error) {
	if transport == nil {
		return nil, errors.New("emulation: transport is nil")
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
		transport: transport,
		reader:    bufio.NewReader(transport),
		timeout:   configuration.timeout,
		tapDelay:  configuration.tapDelay,
		responses: make(chan responseResult, 4),
		done:      make(chan struct{}),
	}
	go client.readResponses()
	return client, nil
}

func (client *Client) readResponses() {
	defer close(client.done)
	defer close(client.responses)
	for {
		frame, err := readFrame(client.reader, responseMagic)
		if err != nil {
			if !client.closed.Load() {
				client.responses <- responseResult{err: err}
			}
			return
		}
		client.responses <- responseResult{frame: frame}
	}
}

func (client *Client) transact(ctx context.Context, operation opcode, payload []byte) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("emulation: context is nil")
	}
	client.commandMu.Lock()
	defer client.commandMu.Unlock()

	if client.closed.Load() {
		return nil, ErrClosed
	}
	client.sequence++
	sequence := client.sequence
	request, err := encodeFrame(requestMagic, sequence, byte(operation), payload)
	if err != nil {
		return nil, err
	}
	if err := writeAll(client.transport, request); err != nil {
		return nil, fmt.Errorf("emulation: write command: %w", err)
	}

	timer := time.NewTimer(client.timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, fmt.Errorf("emulation: command 0x%02X timed out after %s", operation, client.timeout)
		case result, ok := <-client.responses:
			if !ok {
				return nil, ErrClosed
			}
			if result.err != nil {
				return nil, fmt.Errorf("emulation: read response: %w", result.err)
			}
			if result.frame.sequence != sequence {
				continue
			}
			if status(result.frame.code) != statusOK {
				return nil, &DeviceError{Status: result.frame.code}
			}
			return result.frame.payload, nil
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
		return Info{}, fmt.Errorf("emulation: malformed info response: got %d bytes", len(payload))
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
		return Info{}, fmt.Errorf("emulation: firmware protocol %d does not match client protocol %d", info.ProtocolVersion, protocolVersion)
	}
	return info, nil
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
		return errors.New("emulation: at least one key is required")
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
		if key == 0 {
			return errors.New("emulation: key 0 is not valid")
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("emulation: duplicate key 0x%02X", byte(key))
		}
		seen[key] = struct{}{}
		if !isModifier(key) {
			nonModifiers++
		}
	}
	if nonModifiers > 6 {
		return errors.New("emulation: a USB boot keyboard supports at most six non-modifier keys at once")
	}
	return nil
}

func isModifier(key Key) bool {
	return key >= KeyLeftCtrl && key <= KeyRightGUI
}

// Press presses a key or chord, waits for the configured tap delay, and then
// releases the keys in reverse order.
func (client *Client) Press(ctx context.Context, keys ...Key) (err error) {
	if err = client.KeyDown(ctx, keys...); err != nil {
		cleanupErr := client.ReleaseKeyboard(context.WithoutCancel(ctx))
		return errors.Join(err, cleanupErr)
	}
	defer func() {
		releaseErr := client.KeyUp(context.WithoutCancel(ctx), reverseKeys(keys)...)
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
	chunk := make([]byte, 0, maxPayload)
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		_, err := client.transact(ctx, opTypeASCII, chunk)
		chunk = chunk[:0]
		return err
	}

	for _, r := range text {
		var special Key
		switch r {
		case '\n', '\r':
			special = KeyEnter
		case '\t':
			special = KeyTab
		case '\b':
			special = KeyBackspace
		default:
			if r < 0x20 || r > 0x7E {
				return fmt.Errorf("emulation: cannot type %q: USB HID text is limited to US-ASCII", r)
			}
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
	if x > 32767 || y > 32767 {
		return errors.New("emulation: absolute coordinates must be between 0 and 32767")
	}
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[0:2], x)
	binary.LittleEndian.PutUint16(payload[2:4], y)
	_, err := client.transact(ctx, opMouseAbs, payload)
	return err
}

// Scroll scrolls vertically and horizontally. Positive vertical values scroll
// up; positive horizontal values scroll right.
func (client *Client) Scroll(ctx context.Context, vertical, horizontal int) error {
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
		return 0, errors.New("emulation: at least one mouse button is required")
	}
	var mask Button
	for _, button := range buttons {
		if button == 0 || button & ^Button(0x1F) != 0 {
			return 0, fmt.Errorf("emulation: invalid mouse button mask 0x%02X", byte(button))
		}
		mask |= button
	}
	return mask, nil
}

// Click clicks one or more buttons simultaneously.
func (client *Client) Click(ctx context.Context, buttons ...Button) (err error) {
	if err = client.ButtonDown(ctx, buttons...); err != nil {
		cleanupErr := client.ReleaseMouse(context.WithoutCancel(ctx))
		return errors.Join(err, cleanupErr)
	}
	defer func() {
		releaseErr := client.ButtonUp(context.WithoutCancel(ctx), buttons...)
		err = errors.Join(err, releaseErr)
	}()
	return waitContext(ctx, client.tapDelay)
}

// DoubleClick performs two left-button clicks using the supplied interval.
func (client *Client) DoubleClick(ctx context.Context, interval time.Duration) error {
	if interval < 0 {
		return errors.New("emulation: double-click interval cannot be negative")
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
		return errors.New("emulation: USB detach duration must be between 250ms and 30s")
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
	if client.closed.Load() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	_, _ = client.transact(ctx, opReleaseAll, nil)
	cancel()
	return client.shutdown()
}

func (client *Client) shutdown() error {
	var closeErr error
	client.closeOnce.Do(func() {
		client.closed.Store(true)
		closeErr = client.transport.Close()
	})
	return closeErr
}

func waitContext(ctx context.Context, delay time.Duration) error {
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
