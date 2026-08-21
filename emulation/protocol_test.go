package emulation

import (
	"bufio"
	"bytes"
	"errors"
	"testing"
)

func TestCRC8SMBUSCheckValue(t *testing.T) {
	if got, want := crc8([]byte("123456789")), byte(0xF4); got != want {
		t.Fatalf("crc8 check value = 0x%02X, want 0x%02X", got, want)
	}
}

func TestFrameRoundTripAndSynchronization(t *testing.T) {
	encoded, err := encodeFrame(requestMagic, 42, byte(opMouseMove), []byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	stream := append([]byte{0, 1, 2, responseMagic}, encoded...)
	frame, err := readFrame(bufio.NewReader(bytes.NewReader(stream)), requestMagic)
	if err != nil {
		t.Fatal(err)
	}
	if frame.sequence != 42 || opcode(frame.code) != opMouseMove || !bytes.Equal(frame.payload, []byte{1, 2, 3, 4}) {
		t.Fatalf("unexpected decoded frame: %#v", frame)
	}
}

func TestFrameRejectsBadChecksum(t *testing.T) {
	encoded, err := encodeFrame(requestMagic, 1, byte(opPing), nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded[len(encoded)-1] ^= 0xFF
	_, err = readFrame(bufio.NewReader(bytes.NewReader(encoded)), requestMagic)
	if !errors.Is(err, errBadChecksum) {
		t.Fatalf("readFrame error = %v, want checksum error", err)
	}
}

func TestFrameRejectsBadVersion(t *testing.T) {
	encoded, err := encodeFrame(requestMagic, 1, byte(opPing), nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded[1]++
	_, err = readFrame(bufio.NewReader(bytes.NewReader(encoded)), requestMagic)
	if !errors.Is(err, errBadVersion) {
		t.Fatalf("readFrame error = %v, want version error", err)
	}
}

func TestEncodeFrameRejectsLargePayload(t *testing.T) {
	_, err := encodeFrame(requestMagic, 1, byte(opPing), make([]byte, maxPayload+1))
	if !errors.Is(err, errFrameLarge) {
		t.Fatalf("encodeFrame error = %v, want payload-size error", err)
	}
}
