package httpbridge

import (
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

func NewServer(config rest.RestConf) *rest.Server {
	config.Middlewares.Log = false
	server := rest.MustNewServer(config)
	server.Use(SafeAccessLog)
	return server
}

func SafeAccessLog(next http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		response := &statusWriter{ResponseWriter: writer}
		completed := false
		defer func() {
			statusCode := response.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
				if !completed {
					statusCode = http.StatusInternalServerError
				}
			}
			logx.WithContext(request.Context()).WithDuration(time.Since(started)).Infow("http request",
				logx.Field("method", request.Method), logx.Field("status", statusCode))
		}()
		next(response, request)
		completed = true
	}
}

type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (writer *statusWriter) WriteHeader(statusCode int) {
	if statusCode >= 100 && statusCode < 200 {
		writer.ResponseWriter.WriteHeader(statusCode)
		return
	}
	if writer.statusCode != 0 {
		return
	}
	writer.statusCode = statusCode
	writer.ResponseWriter.WriteHeader(statusCode)
}

func (writer *statusWriter) Write(body []byte) (int, error) {
	if writer.statusCode == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(body)
}

func (writer *statusWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}
