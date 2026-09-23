package checker

import (
	"bufio"
	"context"
	"errors"
	"io"
)

type Mode string

const (
	Exact              Mode  = "exact"
	TrailingWhitespace Mode  = "trailing_whitespace"
	Tokens             Mode  = "tokens"
	MaxBytes           int64 = 512 << 20
)

var ErrLimit = errors.New("checker input exceeds bounded byte limit")

type boundedReader struct {
	ctx       context.Context
	reader    io.Reader
	remaining int64
}

func (reader *boundedReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	count, err := reader.reader.Read(buffer[:min(int64(len(buffer)), reader.remaining+1)])
	reader.remaining -= int64(count)
	if reader.remaining < 0 {
		return 0, ErrLimit
	}
	return count, err
}

func Compare(ctx context.Context, mode Mode, expected, actual io.Reader) (bool, error) {
	if expected == nil || actual == nil {
		return false, errors.New("checker readers are required")
	}
	left := bufio.NewReaderSize(&boundedReader{ctx: ctx, reader: expected, remaining: MaxBytes}, 32<<10)
	right := bufio.NewReaderSize(&boundedReader{ctx: ctx, reader: actual, remaining: MaxBytes}, 32<<10)
	switch mode {
	case Exact:
		for {
			first, firstErr := left.ReadByte()
			second, secondErr := right.ReadByte()
			if err := readError(firstErr, secondErr); err != nil {
				return false, err
			}
			if firstErr == io.EOF || secondErr == io.EOF {
				return firstErr == secondErr, nil
			}
			if first != second {
				return false, nil
			}
		}
	case Tokens:
		for {
			first, firstErr := nonWhitespace(left)
			second, secondErr := nonWhitespace(right)
			if err := readError(firstErr, secondErr); err != nil {
				return false, err
			}
			if firstErr == io.EOF || secondErr == io.EOF {
				return firstErr == secondErr, nil
			}
			for {
				if first != second {
					return false, nil
				}
				first, firstErr = left.ReadByte()
				second, secondErr = right.ReadByte()
				if err := readError(firstErr, secondErr); err != nil {
					return false, err
				}
				firstEnd, secondEnd := firstErr == io.EOF || whitespace(first), secondErr == io.EOF || whitespace(second)
				if firstEnd || secondEnd {
					if firstEnd != secondEnd {
						return false, nil
					}
					break
				}
			}
		}
	case TrailingWhitespace:
		for {
			first, firstErr := left.ReadByte()
			second, secondErr := right.ReadByte()
			if err := readError(firstErr, secondErr); err != nil {
				return false, err
			}
			if firstErr == io.EOF && secondErr == io.EOF {
				return true, nil
			}
			if firstErr == nil && secondErr == nil && first == second {
				continue
			}
			if !lineTail(first, firstErr) || !lineTail(second, secondErr) {
				return false, nil
			}
			firstOK, err := consumeTail(left, first, firstErr)
			if err != nil {
				return false, err
			}
			secondOK, err := consumeTail(right, second, secondErr)
			if err != nil {
				return false, err
			}
			if !firstOK || !secondOK {
				return false, nil
			}
		}
	default:
		return false, errors.New("unsupported standard checker mode")
	}
}

func whitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '\r' || value == '\v' || value == '\f'
}
func lineTail(value byte, err error) bool {
	return err == io.EOF || value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func nonWhitespace(reader *bufio.Reader) (byte, error) {
	for {
		value, err := reader.ReadByte()
		if err != nil || !whitespace(value) {
			return value, err
		}
	}
}

func consumeTail(reader *bufio.Reader, value byte, err error) (bool, error) {
	for err != io.EOF && value != '\n' {
		if !lineTail(value, err) {
			return false, nil
		}
		value, err = reader.ReadByte()
		if failure := readError(err); failure != nil {
			return false, failure
		}
	}
	return true, nil
}

func readError(failures ...error) error {
	for _, err := range failures {
		if err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}
