package httpbridge

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const authorizationMetadataKey = "authorization"

func ProtectResponse(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next(w, r)
	}
}

// ForwardAuthorization 只把用户 Authorization 传给下游 RPC。
// 严禁在 HTTP 服务中伪造 x-service-name/x-service-token，避免绕过 RPC 权限边界。
func ForwardAuthorization(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authorization := strings.TrimSpace(r.Header.Get("Authorization"))
		if authorization == "" {
			next(w, r)
			return
		}
		md, _ := metadata.FromOutgoingContext(r.Context())
		md = md.Copy()
		md.Set(authorizationMetadataKey, authorization)
		ctx := metadata.NewOutgoingContext(r.Context(), md)
		next(w, r.WithContext(ctx))
	}
}

// BearerToken 严格提取 Bearer Token，仅供 HTTP 身份中间件做提前校验。
func BearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ConfigureErrorHandler 将 gRPC 状态统一映射为 HTTP 状态码和 JSON 错误体。
func ConfigureErrorHandler() {
	httpx.SetErrorHandlerCtx(func(_ context.Context, err error) (int, any) {
		if grpcStatus, ok := status.FromError(err); ok {
			return grpcCodeToHTTP(grpcStatus.Code()), ErrorResponse{
				Code: strings.ToLower(grpcStatus.Code().String()), Message: grpcStatus.Message(),
			}
		}
		return http.StatusBadRequest, ErrorResponse{Code: "bad_request", Message: err.Error()}
	})
}

func grpcCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.InvalidArgument, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists, codes.Aborted:
		return http.StatusConflict
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func EnumName(value, prefix string) string {
	return strings.ToLower(strings.TrimPrefix(value, prefix))
}

func Timestamp(value *timestamppb.Timestamp) *string {
	if value == nil || !value.IsValid() {
		return nil
	}
	formatted := value.AsTime().UTC().Format(time.RFC3339Nano)
	return &formatted
}

func ParseTimestamp(value, field string) (*timestamppb.Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s must be RFC3339", field)
	}
	timestamp := timestamppb.New(parsed.UTC())
	if err := timestamp.CheckValid(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%s must be a valid protobuf timestamp", field)
	}
	return timestamp, nil
}

func ParseOptionalTimestamp(value *string, field string) (*timestamppb.Timestamp, error) {
	if value == nil {
		return nil, nil
	}
	return ParseTimestamp(*value, field)
}

func DurationSeconds(value *durationpb.Duration) int64 {
	if value == nil || !value.IsValid() {
		return 0
	}
	return int64(value.AsDuration() / time.Second)
}
