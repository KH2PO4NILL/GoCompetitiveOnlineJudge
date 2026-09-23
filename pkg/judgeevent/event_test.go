package judgeevent

import (
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEventWireValidation(t *testing.T) {
	event := &pb.JudgeTaskEvent{SchemaVersion: 1, EventId: uuid.NewString(), SubmissionId: 1, AttemptId: 2, EnqueuedAt: timestamppb.New(time.Now())}
	content, err := Encode(event)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(content, "1")
	if err != nil || !proto.Equal(event, decoded) {
		t.Fatal(err)
	}
	for _, input := range [][]byte{nil, {255}, make([]byte, MaxBytes+1)} {
		if _, err := Decode(input, "1"); err == nil {
			t.Fatal("invalid bytes accepted")
		}
	}
	if _, err := Decode(content, "2"); err == nil {
		t.Fatal("wrong key accepted")
	}
	event.SchemaVersion = 2
	if _, err := Encode(event); err == nil {
		t.Fatal("unsupported schema accepted")
	}
}
