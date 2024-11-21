package config

// RateLimiter contains configuration for rate limiting.
type RateLimiter struct {
    Rps     float64
    Burst   int
    Enabled bool
}