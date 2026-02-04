// Package logging provides structured logging utilities.
package logging

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"context"
)

// NewLogger creates a new zerolog logger configured for the service.
func NewLogger(serviceName string, level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	return zerolog.New(os.Stdout).
		Level(lvl).
		With().
		Timestamp().
		Str("service", serviceName).
		Logger()
}

// NewPrettyLogger creates a logger with pretty console output (for development).
func NewPrettyLogger(serviceName string, level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	return zerolog.New(consoleWriter).
		Level(lvl).
		With().
		Timestamp().
		Str("service", serviceName).
		Logger()
}

// RequestLogger returns a Gin middleware for HTTP request logging.
func RequestLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get client IP
		clientIP := c.ClientIP()

		// Get status code
		statusCode := c.Writer.Status()

		// Get request ID if present
		requestID := c.GetHeader("X-Request-ID")

		// Build log event
		event := logger.Info()
		if statusCode >= 400 && statusCode < 500 {
			event = logger.Warn()
		} else if statusCode >= 500 {
			event = logger.Error()
		}

		event.
			Str("type", "http_request").
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", raw).
			Int("status", statusCode).
			Str("clientIp", clientIP).
			Dur("latency", latency).
			Int("bodySize", c.Writer.Size()).
			Str("userAgent", c.Request.UserAgent())

		if requestID != "" {
			event.Str("requestId", requestID)
		}

		// Add error if present
		if len(c.Errors) > 0 {
			event.Str("error", c.Errors.String())
		}

		event.Msg("HTTP request")
	}
}

// GRPCLogger returns a gRPC unary server interceptor for request logging.
func GRPCLogger(logger zerolog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Call the handler
		resp, err := handler(ctx, req)

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		code := codes.OK
		if err != nil {
			if s, ok := status.FromError(err); ok {
				code = s.Code()
			} else {
				code = codes.Unknown
			}
		}

		// Build log event
		event := logger.Info()
		if code != codes.OK {
			event = logger.Error()
		}

		event.
			Str("type", "grpc_request").
			Str("method", info.FullMethod).
			Str("code", code.String()).
			Dur("latency", latency)

		if err != nil {
			event.Err(err)
		}

		event.Msg("gRPC request")

		return resp, err
	}
}

// GRPCStreamLogger returns a gRPC stream server interceptor for request logging.
func GRPCStreamLogger(logger zerolog.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()

		// Call the handler
		err := handler(srv, ss)

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		code := codes.OK
		if err != nil {
			if s, ok := status.FromError(err); ok {
				code = s.Code()
			} else {
				code = codes.Unknown
			}
		}

		// Build log event
		event := logger.Info()
		if code != codes.OK {
			event = logger.Error()
		}

		event.
			Str("type", "grpc_stream").
			Str("method", info.FullMethod).
			Bool("clientStream", info.IsClientStream).
			Bool("serverStream", info.IsServerStream).
			Str("code", code.String()).
			Dur("latency", latency)

		if err != nil {
			event.Err(err)
		}

		event.Msg("gRPC stream")

		return err
	}
}

// ContextWithLogger adds a logger to the context.
func ContextWithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return logger.WithContext(ctx)
}

// LoggerFromContext extracts the logger from context.
func LoggerFromContext(ctx context.Context) zerolog.Logger {
	return *zerolog.Ctx(ctx)
}

// NotificationLogger creates a logger specifically for notification operations.
func NotificationLogger(logger zerolog.Logger, notificationID string, channel string) zerolog.Logger {
	return logger.With().
		Str("notificationId", notificationID).
		Str("channel", channel).
		Logger()
}

// AlertLogger creates a logger specifically for alert operations.
func AlertLogger(logger zerolog.Logger, alertID string, severity string) zerolog.Logger {
	return logger.With().
		Str("alertId", alertID).
		Str("severity", severity).
		Logger()
}

// HTTPMiddleware returns a standard http.Handler middleware for logging.
func HTTPMiddleware(logger zerolog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Process request
		next.ServeHTTP(rw, r)

		// Calculate latency
		latency := time.Since(start)

		// Build log event
		event := logger.Info()
		if rw.statusCode >= 400 && rw.statusCode < 500 {
			event = logger.Warn()
		} else if rw.statusCode >= 500 {
			event = logger.Error()
		}

		event.
			Str("type", "http_request").
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("query", r.URL.RawQuery).
			Int("status", rw.statusCode).
			Str("remoteAddr", r.RemoteAddr).
			Dur("latency", latency).
			Str("userAgent", r.UserAgent()).
			Msg("HTTP request")
	})
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
