package model

import (
	"strings"
	"testing"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgepolicy"
)

func TestCreateSourceSizeMatchesJudgePolicy(t *testing.T) {
	for _, length := range []int{judgepolicy.MaxSourceCodeBytes, judgepolicy.MaxSourceCodeBytes + 1, 4 << 20} {
		input := CreateSubmissionInput{UserID: 1, ProblemID: 1, ProblemRevision: 1, Language: "go", ClientRequestID: "size-boundary", SourceCode: strings.Repeat("x", length)}
		err := validateCreateInput(&input)
		if (err != nil) != (length > judgepolicy.MaxSourceCodeBytes) {
			t.Fatalf("size %d: %v", length, err)
		}
	}
}
