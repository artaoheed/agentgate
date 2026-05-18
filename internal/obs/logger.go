package obs

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey int

const loggerCtxKey ctxKey = 0

// NewLogger returns a slog.Logger configured for the current ENV. JSON
// in prod (default), text when ENV=dev — easier to read locally.
func NewLogger() *slog.Logger {
	var handler slog.Handler
	if os.Getenv("ENV") == "dev" {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}
	return slog.New(handler)
}

// WithLogger attaches a logger to ctx so per-request fields propagate.
func WithLogger(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey, log)
}

// LoggerFrom returns the request-scoped logger if one is attached,
// otherwise the package default.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerCtxKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
