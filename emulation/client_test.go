package emulation

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type harnessResponse struct {
	status  status
	payload []byte
	respond bool
}

type clientHarness struct {
	client   *Client
	server   net.Conn
	commands chan wireFrame
	done     chan struct{}
}

func newClientHarness(t *testing.T, handler func(wireFrame) harnessResponse, options ...Option) *clientHarness {
	t.Helper()
	clientSide, serverSide := net.Pipe()
	client, err := NewClient(clientSide, options...)
	if err != nil {
		t.Fatal(err)
	}
	harness := &clientHarness{
		client:   client,
		server:   serverSide,
		commands: make(chan wireFrame, 128),
		done:     make(chan struct{}),
	}
	go func() {
		defer close(harness.done)
		reader := bufio.NewReader(serverSide)
		for {
			request, err := readFrame(reader, requestMagic)
			if err != nil {
				return
			}
			harness.commands <- request
			response := harnessResponse{respond: true}
			if handler != nil {
				response = handler(request)
			}
			if !response.respond {
				continue
			}
			encoded, err := encodeFrame(responseMagic, request.sequence, byte(response.status), response.payload)
			if err != nil || writeAll(serverSide, encoded) != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = harness.client.shutdown()
		_ = harness.server.Close()
		<-harness.done
	})
	return harness
}

func (harness *clientHarness) next(t *testing.T) wireFrame {
	t.Helper()
	select {
	case command := <-harness.commands:
		return command
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for command")
		return wireFrame{}
	}
}

func assertCommand(t *testing.T, frame wireFrame, operation opcode, payload []byte) {
	t.Helper()
	if opcode(frame.code) != operation || !bytes.Equal(frame.payload, payload) {
		t.Fatalf("command = {op: 0x%02X, payload: %v}, want {op: 0x%02X, payload: %v}", frame.code, frame.payload, operation, payload)
	}
}

func TestInfoAndPing(t *testing.T) {
	infoPayload := []byte{1, 7, protocolVersion, 0x1F, 0, maxPayload, 30}
	harness := newClientHarness(t, func(request wireFrame) harnessResponse {
		if opcode(request.code) == opInfo {
			return harnessResponse{respond: true, payload: infoPayload}
		}
		return harnessResponse{respond: true}
	})
	ctx := context.Background()

	info, err := harness.client.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.FirmwareMajor != 1 || info.FirmwareMinor != 7 || info.ProtocolVersion != protocolVersion ||
		info.Capabilities != 0x1F || info.MaximumPayload != maxPayload || info.WatchdogTimeout != 30*time.Second {
		t.Fatalf("unexpected info: %#v", info)
	}
	if err := harness.client.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opInfo, nil)
	assertCommand(t, harness.next(t), opPing, nil)
}

func TestDeviceError(t *testing.T) {
	harness := newClientHarness(t, func(wireFrame) harnessResponse {
		return harnessResponse{respond: true, status: statusBadPayload}
	})
	err := harness.client.Ping(context.Background())
	var deviceErr *DeviceError
	if !errors.As(err, &deviceErr) || deviceErr.Status != byte(statusBadPayload) {
		t.Fatalf("Ping error = %v, want bad-payload DeviceError", err)
	}
}

func TestCommandTimeout(t *testing.T) {
	harness := newClientHarness(t, func(wireFrame) harnessResponse {
		return harnessResponse{respond: false}
	}, WithTimeout(20*time.Millisecond))
	if err := harness.client.Ping(context.Background()); err == nil {
		t.Fatal("Ping succeeded without a response")
	}
}

func TestPressReleasesChordInReverseOrder(t *testing.T) {
	harness := newClientHarness(t, nil, WithTapDelay(0))
	if err := harness.client.Press(context.Background(), Ctrl, MustKey('a')); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opKeyDown, []byte{byte(Ctrl), 'a'})
	assertCommand(t, harness.next(t), opKeyUp, []byte{'a', byte(Ctrl)})
}

func TestKeyValidation(t *testing.T) {
	harness := newClientHarness(t, nil)
	ctx := context.Background()
	if err := harness.client.KeyDown(ctx, MustKey('a'), MustKey('a')); err == nil {
		t.Fatal("KeyDown accepted duplicate keys")
	}
	if err := harness.client.KeyDown(ctx, 0); err == nil {
		t.Fatal("KeyDown accepted key zero")
	}
	if err := harness.client.KeyDown(ctx,
		MustKey('a'), MustKey('b'), MustKey('c'), MustKey('d'),
		MustKey('e'), MustKey('f'), MustKey('g')); err == nil {
		t.Fatal("KeyDown accepted seven non-modifier keys")
	}
	if err := harness.client.KeyDown(ctx,
		Ctrl, Shift, Alt, GUI, KeyRightCtrl, KeyRightShift, KeyRightAlt,
		KeyRightGUI, MustKey('a'), MustKey('b'), MustKey('c'), MustKey('d'),
		MustKey('e'), MustKey('f')); err != nil {
		t.Fatalf("KeyDown rejected a valid six-key chord: %v", err)
	}
	assertCommand(t, harness.next(t), opKeyDown, []byte{
		byte(Ctrl), byte(Shift), byte(Alt), byte(GUI), byte(KeyRightCtrl),
		byte(KeyRightShift), byte(KeyRightAlt), byte(KeyRightGUI), 'a', 'b', 'c', 'd', 'e', 'f',
	})
}

func TestPressReleasesAfterCancellation(t *testing.T) {
	harness := newClientHarness(t, func(request wireFrame) harnessResponse {
		if opcode(request.code) == opKeyDown {
			time.Sleep(20 * time.Millisecond)
		}
		return harnessResponse{respond: true}
	}, WithTapDelay(time.Hour))
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- harness.client.Press(ctx, MustKey('x')) }()
	assertCommand(t, harness.next(t), opKeyDown, []byte{'x'})
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Press error = %v, want context canceled", err)
	}
	assertCommand(t, harness.next(t), opKeyboardReset, nil)
}

func TestTypeChunksTextAndHandlesControlKeys(t *testing.T) {
	harness := newClientHarness(t, nil, WithTapDelay(0))
	text := string(bytes.Repeat([]byte{'a'}, maxPayload+1)) + "\n\t\b"
	if err := harness.client.Type(context.Background(), text); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opTypeASCII, bytes.Repeat([]byte{'a'}, maxPayload))
	assertCommand(t, harness.next(t), opTypeASCII, []byte{'a'})
	assertCommand(t, harness.next(t), opKeyDown, []byte{byte(KeyEnter)})
	assertCommand(t, harness.next(t), opKeyUp, []byte{byte(KeyEnter)})
	assertCommand(t, harness.next(t), opKeyDown, []byte{byte(KeyTab)})
	assertCommand(t, harness.next(t), opKeyUp, []byte{byte(KeyTab)})
	assertCommand(t, harness.next(t), opKeyDown, []byte{byte(KeyBackspace)})
	assertCommand(t, harness.next(t), opKeyUp, []byte{byte(KeyBackspace)})
}

func TestTypeRejectsUnicodeBeforeSending(t *testing.T) {
	harness := newClientHarness(t, nil)
	if err := harness.client.Type(context.Background(), "ok\u4e16"); err == nil {
		t.Fatal("Type accepted non-ASCII text")
	}
	// The valid prefix is buffered until the entire operation can continue, so
	// rejecting the unsupported rune produces no partial typing.
	select {
	case command := <-harness.commands:
		t.Fatalf("unexpected partial command: %#v", command)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestRelativeMoveAndScrollChunking(t *testing.T) {
	harness := newClientHarness(t, nil)
	ctx := context.Background()
	if err := harness.client.Move(ctx, 300, -300); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opMouseMove, []byte{127, 129, 0, 0})
	assertCommand(t, harness.next(t), opMouseMove, []byte{127, 129, 0, 0})
	assertCommand(t, harness.next(t), opMouseMove, []byte{46, 210, 0, 0})

	if err := harness.client.Scroll(ctx, -128, 255); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opMouseMove, []byte{0, 0, 129, 127})
	assertCommand(t, harness.next(t), opMouseMove, []byte{0, 0, 255, 127})
	assertCommand(t, harness.next(t), opMouseMove, []byte{0, 0, 0, 1})
}

func TestAbsoluteMovement(t *testing.T) {
	harness := newClientHarness(t, nil)
	if err := harness.client.MoveAbsolute(context.Background(), 1234, 32767); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 4)
	binary.LittleEndian.PutUint16(payload[:2], 1234)
	binary.LittleEndian.PutUint16(payload[2:], 32767)
	assertCommand(t, harness.next(t), opMouseAbs, payload)
	if err := harness.client.MoveAbsolute(context.Background(), 32768, 0); err == nil {
		t.Fatal("MoveAbsolute accepted an out-of-range coordinate")
	}
}

func TestMouseButtonsAndClick(t *testing.T) {
	harness := newClientHarness(t, nil, WithTapDelay(0))
	ctx := context.Background()
	if err := harness.client.ButtonDown(ctx, ButtonLeft, ButtonForward); err != nil {
		t.Fatal(err)
	}
	if err := harness.client.ButtonUp(ctx, ButtonForward); err != nil {
		t.Fatal(err)
	}
	if err := harness.client.Click(ctx, ButtonRight); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opMouseDown, []byte{0x11})
	assertCommand(t, harness.next(t), opMouseUp, []byte{0x10})
	assertCommand(t, harness.next(t), opMouseDown, []byte{0x02})
	assertCommand(t, harness.next(t), opMouseUp, []byte{0x02})
	if err := harness.client.ButtonDown(ctx, Button(0x80)); err == nil {
		t.Fatal("ButtonDown accepted an invalid button")
	}
}

func TestResetCommands(t *testing.T) {
	harness := newClientHarness(t, nil)
	ctx := context.Background()
	if err := harness.client.ReleaseKeyboard(ctx); err != nil {
		t.Fatal(err)
	}
	if err := harness.client.ReleaseMouse(ctx); err != nil {
		t.Fatal(err)
	}
	if err := harness.client.ReleaseAll(ctx); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opKeyboardReset, nil)
	assertCommand(t, harness.next(t), opMouseReset, nil)
	assertCommand(t, harness.next(t), opReleaseAll, nil)
}

func TestCycleUSBClosesClient(t *testing.T) {
	harness := newClientHarness(t, nil)
	if err := harness.client.CycleUSB(context.Background(), 500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 2)
	binary.LittleEndian.PutUint16(payload, 500)
	assertCommand(t, harness.next(t), opCycleUSB, payload)
	if err := harness.client.Ping(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Ping after CycleUSB = %v, want ErrClosed", err)
	}
}

func TestCloseSendsReleaseAll(t *testing.T) {
	harness := newClientHarness(t, nil)
	if err := harness.client.Close(); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opReleaseAll, nil)
	if err := harness.client.Close(); err != nil {
		t.Fatalf("second Close = %v", err)
	}
}

func TestConcurrentCommandsAreSerialized(t *testing.T) {
	harness := newClientHarness(t, nil)
	const count = 20
	var group sync.WaitGroup
	errorsFound := make(chan error, count)
	for range count {
		group.Add(1)
		go func() {
			defer group.Done()
			errorsFound <- harness.client.Ping(context.Background())
		}()
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := make(map[byte]bool, count)
	for range count {
		command := harness.next(t)
		if seen[command.sequence] {
			t.Fatalf("duplicate sequence %d", command.sequence)
		}
		seen[command.sequence] = true
	}
}

func TestReadFailureIsReported(t *testing.T) {
	clientSide, serverSide := net.Pipe()
	client, err := NewClient(clientSide, WithTimeout(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	defer client.shutdown()
	result := make(chan error, 1)
	go func() { result <- client.Ping(context.Background()) }()
	request := make([]byte, 6)
	if _, err := io.ReadFull(serverSide, request); err != nil {
		t.Fatal(err)
	}
	_ = serverSide.Close()
	if err := <-result; err == nil {
		t.Fatal("Ping succeeded after the transport closed")
	}
}
