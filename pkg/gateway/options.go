package gateway

import (
	"log/slog"
	"os"
)

// clientOptions holds configuration options for the Client.
type clientOptions struct {
	logger       *slog.Logger
	cacheEnabled bool
	// TODO: Add hooks when Hook system is implemented (Sprint 5)
}

// defaultOptions returns the default client options.
func defaultOptions() *clientOptions {
	return &clientOptions{
		logger:       slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
		cacheEnabled: true,
	}
}

// Option is a function that configures the Client.
type Option func(*clientOptions)

// WithLogger sets a custom logger for the client.
func WithLogger(logger *slog.Logger) Option {
	return func(o *clientOptions) {
		if logger != nil {
			o.logger = logger
		}
	}
}

// WithCache enables or disables caching.
func WithCache(enabled bool) Option {
	return func(o *clientOptions) {
		o.cacheEnabled = enabled
	}
}

// WithDebug enables debug logging.
func WithDebug() Option {
	return func(o *clientOptions) {
		o.logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
}

// TODO: WithHook will be implemented in Sprint 5
// func WithHook(h hook.Hook) Option {
//     return func(o *clientOptions) {
//         o.hooks = append(o.hooks, h)
//     }
// }
