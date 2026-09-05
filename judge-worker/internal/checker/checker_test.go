package checker

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestStandardModes(t *testing.T) {
	for _, test := range []struct {
		mode        Mode
		left, right string
		equal       bool
	}{
		{Exact, "", "", true}, {Exact, "42\n", "42", false}, {Exact, "a\x00b", "a\x00b", true},
		{TrailingWhitespace, "42\n", "42 \t\r\n\n", true}, {TrailingWhitespace, " a\n", "a\n", false},
		{TrailingWhitespace, "a b", "a  b", false}, {TrailingWhitespace, "a\tb", "a b", false},
		{TrailingWhitespace, "a\n b", "a \r\n b \n", true}, {TrailingWhitespace, "a\n\nb", "a\nb", false},
		{TrailingWhitespace, "", " \t\n\n", true}, {TrailingWhitespace, "a ", "a\tx", false},
		{Tokens, " 42\n1\t2", "42 1 2 \n", true}, {Tokens, "12 3", "1 23", false},
		{Tokens, "a", "a b", false}, {Tokens, "", "\n\t", true}, {Tokens, "a\x00b", "a b", false},
		{Tokens, strings.Repeat("a", 100000), strings.Repeat("a", 100000), true},
	} {
		equal, err := Compare(context.Background(), test.mode, strings.NewReader(test.left), strings.NewReader(test.right))
		if err != nil || equal != test.equal {
			t.Fatalf("%s %q %q: %v %v", test.mode, test.left, test.right, equal, err)
		}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestCheckerErrorsAndCancellation(t *testing.T) {
	for _, mode := range []Mode{Exact, TrailingWhitespace, Tokens} {
		if _, err := Compare(context.Background(), mode, failingReader{}, strings.NewReader("")); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := Compare(ctx, mode, strings.NewReader("a"), strings.NewReader("a")); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	reader := &boundedReader{ctx: context.Background(), reader: strings.NewReader("abcd"), remaining: 3}
	if _, err := io.ReadAll(reader); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}
