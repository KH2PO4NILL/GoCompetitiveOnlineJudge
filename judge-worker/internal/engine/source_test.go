package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgepolicy"
)

func TestInvalidSourceProducesTerminalCompileError(t *testing.T) {
	for _, sources := range []map[string][]byte{
		{"main.fake": []byte(strings.Repeat("x", judgepolicy.MaxSourceCodeBytes+1))},
		{"other.fake": []byte("missing entry")},
		{"main.fake": []byte("entry"), "../other.fake": []byte("invalid path")},
	} {
		engine, executor, input := judgeFixture(t)
		input.Request.Sources = sources
		outcome, err := engine.Judge(context.Background(), input)
		if err != nil || outcome.Verdict != CE || len(outcome.Cases) != 0 || outcome.CompileOutput == "" || outcome.Policy.ProfileID == "" || outcome.Policy.PolicyDigest == "" || len(executor.requests) != 0 {
			t.Fatalf("invalid input did not terminate before execution: %+v error=%v", outcome, err)
		}
	}
}

func TestSourceSizeBoundaryCanBePrepared(t *testing.T) {
	engine, _, request := newTestEngine(t)
	request.Sources["main.fake"] = []byte(strings.Repeat("x", judgepolicy.MaxSourceCodeBytes))
	attempt, err := engine.Prepare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if err := attempt.Close(); err != nil {
		t.Fatal(err)
	}
}
