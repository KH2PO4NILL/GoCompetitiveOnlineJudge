package sandbox

import (
	"encoding/binary"
	"errors"
	"io"
)

type supervisorOutput struct {
	stdout, stderr io.Writer
	pending        []byte
	finished       bool
	cpu, memory    uint64
	exitCode       int64
	flags          uint32
}

func (output *supervisorOutput) Write(data []byte) (int, error) {
	length := len(data)
	for len(data) > 0 {
		if output.finished {
			return 0, errors.New("sandbox supervisor sent data after final accounting")
		}
		target := 5
		if len(output.pending) >= 5 {
			size := binary.BigEndian.Uint32(output.pending[1:5])
			if size > 16384 {
				return 0, errors.New("sandbox supervisor frame exceeds limit")
			}
			target += int(size)
		}
		count := min(target-len(output.pending), len(data))
		output.pending = append(output.pending, data[:count]...)
		data = data[count:]
		if len(output.pending) < 5 || len(output.pending) != 5+int(binary.BigEndian.Uint32(output.pending[1:5])) {
			continue
		}
		payload := output.pending[5:]
		var err error
		switch output.pending[0] {
		case 1:
			_, err = output.stdout.Write(payload)
		case 2:
			_, err = output.stderr.Write(payload)
		case 3:
			if len(payload) != 24 {
				return 0, errors.New("invalid sandbox final accounting frame")
			}
			output.cpu = binary.BigEndian.Uint64(payload[:8])
			output.memory = binary.BigEndian.Uint64(payload[8:16])
			output.exitCode = int64(binary.BigEndian.Uint32(payload[16:20]))
			output.flags = binary.BigEndian.Uint32(payload[20:24])
			if output.exitCode > 255 || output.flags > 3 {
				return 0, errors.New("invalid sandbox final status")
			}
			output.finished = true
		default:
			return 0, errors.New("invalid sandbox supervisor frame kind")
		}
		if err != nil {
			return 0, err
		}
		output.pending = output.pending[:0]
	}
	return length, nil
}
