package sandbox

import (
	"errors"
	"sync"
	"testing"
)

func TestBoundedBuffersShareOutputBudget(t *testing.T) {
	budget := &outputBudget{limit: 6}
	stdout := &boundedBuffer{budget: budget}
	stderr := &boundedBuffer{budget: budget}
	if written, err := stdout.Write([]byte("abcd")); written != 4 || err != nil {
		t.Fatalf("stdout.Write() = %d, %v", written, err)
	}
	if written, err := stderr.Write([]byte("WXYZ")); written != 2 || !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("stderr.Write() = %d, %v, want 2 and ErrOutputLimit", written, err)
	}
	if got := string(stdout.Bytes()); got != "abcd" {
		t.Fatalf("stdout = %q", got)
	}
	if got := string(stderr.Bytes()); got != "WX" {
		t.Fatalf("stderr = %q", got)
	}
	if got := budget.bytes(); got != 6 {
		t.Fatalf("output bytes = %d", got)
	}
}

func TestBoundedBufferAllowsOutputExactlyAtLimit(t *testing.T) {
	budget := &outputBudget{limit: 4}
	buffer := &boundedBuffer{budget: budget}
	if written, err := buffer.Write([]byte("abcd")); written != 4 || err != nil {
		t.Fatalf("Write(exact limit) = %d, %v", written, err)
	}
	if written, err := buffer.Write([]byte("x")); written != 0 || !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("Write(over limit) = %d, %v, want 0 and ErrOutputLimit", written, err)
	}
}

func TestBoundedBufferSupportsConcurrentSnapshots(t *testing.T) {
	budget := &outputBudget{limit: 1 << 20}
	buffer := &boundedBuffer{budget: budget}
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for range 1000 {
			_, _ = buffer.Write([]byte("x"))
		}
	}()
	go func() {
		defer wait.Done()
		for range 1000 {
			_ = buffer.Bytes()
		}
	}()
	wait.Wait()
	if got := len(buffer.Bytes()); got != 1000 {
		t.Fatalf("buffer length = %d", got)
	}
}
