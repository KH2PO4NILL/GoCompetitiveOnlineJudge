package judgeevent

import (
	"errors"
	"strconv"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

const (
	SchemaVersion = 1
	Topic         = "judge-task"
	DeadTopic     = "judge-task-dead"
	Group         = "judge-workers"
	MaxBytes      = 16 << 10
)

func Validate(event *pb.JudgeTaskEvent) error {
	if event == nil || event.SchemaVersion != SchemaVersion || event.SubmissionId <= 0 || event.AttemptId <= 0 || event.EnqueuedAt == nil || event.EnqueuedAt.CheckValid() != nil {
		return errors.New("invalid JudgeTaskEvent or unsupported schema version")
	}
	if _, err := uuid.Parse(event.EventId); err != nil || len(event.EventId) != 36 {
		return errors.New("event_id must be a canonical UUID")
	}
	return nil
}
func Encode(event *pb.JudgeTaskEvent) ([]byte, error) {
	if err := Validate(event); err != nil {
		return nil, err
	}
	content, err := proto.MarshalOptions{Deterministic: true}.Marshal(event)
	if len(content) > MaxBytes {
		return nil, errors.New("JudgeTaskEvent exceeds byte limit")
	}
	return content, err
}
func Decode(content []byte, key string) (*pb.JudgeTaskEvent, error) {
	if len(content) == 0 || len(content) > MaxBytes {
		return nil, errors.New("invalid JudgeTaskEvent size")
	}
	var event pb.JudgeTaskEvent
	if err := proto.Unmarshal(content, &event); err != nil {
		return nil, errors.New("malformed JudgeTaskEvent protobuf")
	}
	if err := Validate(&event); err != nil {
		return nil, err
	}
	if key != strconv.FormatInt(event.SubmissionId, 10) {
		return nil, errors.New("message key does not match submission_id")
	}
	return &event, nil
}
