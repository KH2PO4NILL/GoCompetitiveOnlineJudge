package logic

import (
	"context"
	"time"

	infraauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	authpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/model"
)

func principalContext(userID int64, role authpb.UserRole) context.Context {
	return infraauth.WithPrincipal(context.Background(), infraauth.Principal{
		UserID: userID, Role: role, Status: authpb.UserStatus_USER_STATUS_NORMAL,
	})
}

func serviceContext(name string) context.Context {
	return infraauth.WithServicePrincipal(context.Background(), infraauth.ServicePrincipal{Name: name})
}

func testProblemAggregate(problemID, authorID, revision int64) *model.ProblemAggregate {
	now := time.Now().UTC()
	return &model.ProblemAggregate{
		ProblemID: problemID, AuthorID: authorID, Revision: revision,
		Status: model.ProblemStatusDraft, CreatedAt: now, UpdatedAt: now,
		Editable: model.EditableProblem{
			Title: "A + B", Description: "description",
			Difficulty: 1, Visibility: model.ProblemVisibilityPrivate,
			TimeLimitMS: 1000, MemoryLimitMB: 256,
			AllowedLanguages: []string{"go"}, CheckerType: 1,
		},
	}
}
