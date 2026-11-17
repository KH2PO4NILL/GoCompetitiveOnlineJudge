package interceptor

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const AuthenticationDomain = "gcoj.auth"

func AuthenticationError(code codes.Code, reason, message string) error {
	s := status.New(code, message)
	detailed, err := s.WithDetails(&errdetails.ErrorInfo{Domain: AuthenticationDomain, Reason: reason})
	if err != nil {
		return s.Err()
	}
	return detailed.Err()
}
func AuthenticationReason(s *status.Status) string {
	for _, d := range s.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.Domain == AuthenticationDomain {
			switch info.Reason {
			case "access_token_invalid", "session_invalid", "account_unavailable", "refresh_token_invalid":
				return info.Reason
			}
		}
	}
	return ""
}
