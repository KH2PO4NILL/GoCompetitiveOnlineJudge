package svc

import (
	"context"
	"errors"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
)

var (
	ErrTestDataObjectNotFound = errors.New("test data object not found")
	ErrTestDataObjectInvalid  = errors.New("test data object is invalid")
)

// TestDataObject is trusted metadata obtained after inspecting an uploaded
// package. TestCases must come from a validated manifest, not from the client
// finalize request.
type TestDataObject struct {
	ContentLength int64
	SHA256        string
	PackageKey    string
	TestCases     []model.TestCaseInput
}

// TestDataStorage isolates object-store signing and package inspection from
// RPC business logic. A production S3/MinIO implementation can be injected
// without coupling Logic or Model to a specific SDK.
type TestDataStorage interface {
	CreateUploadURL(ctx context.Context, upload *model.TestDataUpload) (string, error)
	InspectUpload(ctx context.Context, upload *model.TestDataUpload) (*TestDataObject, error)
}
