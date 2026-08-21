package hid

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

const (
	protocolVersion byte = 1
	requestMagic    byte = 0xA5
	responseMagic   byte = 0x5A
	maxPayload           = 64
)

type opcode byte

const (
	opPing opcode = 0x01
	opInfo opcode = 0x02

	opKeyDown       opcode = 0x10
	opKeyUp         opcode = 0x11
	opKeyboardReset opcode = 0x12
	opTypeASCII     opcode = 0x13

	opMouseMove  opcode = 0x20
	opMouseAbs   opcode = 0x21
	opMouseDown  opcode = 0x22
	opMouseUp    opcode = 0x23
	opMouseReset opcode = 0x24

	opReleaseAll opcode = 0x30
	opCycleUSB   opcode = 0x31
)

type status byte

const (
	statusOK          status = 0
	statusUnknownOp   status = 1
	statusBadPayload  status = 2
	statusHIDFailure  status = 3
	statusBadChecksum status = 4
	statusBadVersion  status = 5
)

type wireFrame struct {
	sequence byte
	code     byte
	payload  []byte
}

var (
	errBadMagic    = errors.New("hid: invalid frame magic")
	errBadVersion  = errors.New("hid: unsupported protocol version")
	errBadChecksum = errors.New("hid: frame checksum mismatch")
	errFrameLarge  = errors.New("hid: frame payload is too large")
)

func encodeFrame(magic byte, sequence byte, code byte, payload []byte) ([]byte, error) {
	if len(payload) > maxPayload {
		return nil, errFrameLarge
	}

	frame := make([]byte, 0, 6+len(payload))
	frame = append(frame, magic, protocolVersion, sequence, code, byte(len(payload)))
	frame = append(frame, payload...)
	frame = append(frame, crc8(frame))
	return frame, nil
}

func readFrame(reader *bufio.Reader, magic byte) (wireFrame, error) {
	for {
		value, err := reader.ReadByte()
		if err != nil {
			return wireFrame{}, err
		}
		if value == magic {
			break
		}
	}

	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return wireFrame{}, err
	}
	frame := wireFrame{sequence: header[1], code: header[2]}
	if header[0] != protocolVersion {
		return frame, fmt.Errorf("%w: got %d, want %d", errBadVersion, header[0], protocolVersion)
	}
	if header[3] > maxPayload {
		return frame, errFrameLarge
	}

	payloadAndCRC := make([]byte, int(header[3])+1)
	if _, err := io.ReadFull(reader, payloadAndCRC); err != nil {
		return wireFrame{}, err
	}
	checksumInput := append([]byte{magic}, header...)
	checksumInput = append(checksumInput, payloadAndCRC[:len(payloadAndCRC)-1]...)
	if crc8(checksumInput) != payloadAndCRC[len(payloadAndCRC)-1] {
		frame.payload = append([]byte(nil), payloadAndCRC[:len(payloadAndCRC)-1]...)
		return frame, errBadChecksum
	}

	frame.payload = append([]byte(nil), payloadAndCRC[:len(payloadAndCRC)-1]...)
	return frame, nil
}

// CRC-8/SMBUS: polynomial 0x07, initial value 0, no reflection.
func crc8(data []byte) byte {
	var crc byte
	for _, value := range data {
		crc ^= value
		for range 8 {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
