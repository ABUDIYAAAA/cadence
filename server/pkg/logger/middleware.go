package logger

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// HTTPMiddleware logs all incoming HTTP requests as structured slog entries.
func HTTPMiddleware(l *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

			reqID := middleware.GetReqID(r.Context())
			reqLogger := l.With(
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)

			ctx := WithLogger(r.Context(), reqLogger)
			next.ServeHTTP(ww, r.WithContext(ctx))

			duration := time.Since(start)
			status := ww.statusCode

			fields := []any{
				slog.Int("status", status),
				slog.Float64("duration_ms", float64(duration.Microseconds())/1000.0),
				slog.Int("bytes", ww.bytes),
				slog.String("remote_ip", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
			}

			switch {
			case status >= 500:
				reqLogger.Error("HTTP Server Error", fields...)
			case status >= 400:
				reqLogger.Warn("HTTP Client Error", fields...)
			default:
				reqLogger.Info("HTTP Request Completed", fields...)
			}
		})
	}
}
