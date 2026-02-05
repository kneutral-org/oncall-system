// Package middleware provides HTTP middleware for the alerting-system.
package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// PayloadTooLargeError is returned when the request body exceeds the size limit.
type PayloadTooLargeError struct {
	MaxBytes int64
}

func (e *PayloadTooLargeError) Error() string {
	return "request body too large"
}

// PayloadLimitErrorResponse represents the JSON response for payload too large errors.
type PayloadLimitErrorResponse struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	MaxBytes   int64  `json:"maxBytes"`
	StatusCode int    `json:"statusCode"`
}

// PayloadLimit returns a middleware that limits the request body size.
// It wraps the request body with http.MaxBytesReader to enforce the limit.
// If the limit is exceeded, subsequent reads will fail, which is then
// handled by the PayloadLimitErrorHandler middleware.
func PayloadLimit(maxBytes int64, logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if no body expected
		if c.Request.Body == nil || c.Request.ContentLength == 0 {
			c.Next()
			return
		}

		// Check Content-Length header first for early rejection
		if c.Request.ContentLength > maxBytes {
			logOversizedRequest(logger, c, c.Request.ContentLength, maxBytes)
			respondPayloadTooLarge(c, maxBytes)
			return
		}

		// Wrap the body with MaxBytesReader for chunked encoding or when Content-Length is not reliable
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		// Store maxBytes in context for error handler
		c.Set("maxPayloadBytes", maxBytes)

		c.Next()
	}
}

// PayloadLimitErrorHandler returns a middleware that handles MaxBytesError
// from http.MaxBytesReader. It should be placed before PayloadLimit in the middleware chain.
func PayloadLimitErrorHandler(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there were any errors during request processing
		if len(c.Errors) > 0 {
			for _, ginErr := range c.Errors {
				// Check if the error is a MaxBytesError
				var maxBytesErr *http.MaxBytesError
				if errors.As(ginErr.Err, &maxBytesErr) {
					maxBytes, _ := c.Get("maxPayloadBytes")
					maxBytesVal, ok := maxBytes.(int64)
					if !ok {
						maxBytesVal = 0
					}

					logOversizedRequest(logger, c, maxBytesErr.Limit, maxBytesVal)

					// Clear existing errors and send proper response
					c.Errors = c.Errors[:0]
					respondPayloadTooLarge(c, maxBytesVal)
					return
				}
			}
		}
	}
}

// logOversizedRequest logs information about an oversized request attempt.
func logOversizedRequest(logger zerolog.Logger, c *gin.Context, attemptedSize, maxBytes int64) {
	logger.Warn().
		Str("clientIP", c.ClientIP()).
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Int64("attemptedSize", attemptedSize).
		Int64("maxBytes", maxBytes).
		Str("userAgent", c.Request.UserAgent()).
		Msg("oversized request rejected")
}

// respondPayloadTooLarge sends a 413 Payload Too Large response.
func respondPayloadTooLarge(c *gin.Context, maxBytes int64) {
	c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, PayloadLimitErrorResponse{
		Error:      "payloadTooLarge",
		Message:    "request body exceeds the maximum allowed size",
		MaxBytes:   maxBytes,
		StatusCode: http.StatusRequestEntityTooLarge,
	})
}
