package sandbox

import (
	"bytes"
	"sync"
)

type outputBudget struct {
	mu    sync.Mutex
	limit int64
	used  int64
}

func (budget *outputBudget) take(length int) (int, error) {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	remaining := budget.limit - budget.used
	if remaining <= 0 {
		return 0, ErrOutputLimit
	}
	accepted := int64(length)
	if accepted > remaining {
		accepted = remaining
	}
	budget.used += accepted
	if accepted < int64(length) {
		return int(accepted), ErrOutputLimit
	}
	return int(accepted), nil
}

func (budget *outputBudget) bytes() int64 {
	budget.mu.Lock()
	defer budget.mu.Unlock()
	return budget.used
}

type boundedBuffer struct {
	budget *outputBudget
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (writer *boundedBuffer) Write(value []byte) (int, error) {
	accepted, err := writer.budget.take(len(value))
	if accepted > 0 {
		writer.mu.Lock()
		_, _ = writer.buffer.Write(value[:accepted])
		writer.mu.Unlock()
	}
	if err != nil {
		return accepted, err
	}
	return len(value), nil
}

func (writer *boundedBuffer) Bytes() []byte {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return append([]byte(nil), writer.buffer.Bytes()...)
}
