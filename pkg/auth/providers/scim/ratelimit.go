package scim

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// rateLimiter enforces per-provider request rate limits using token bucket algorithm.
// Each provider gets an independent limiter, so one provider's traffic doesn't affect another.
type rateLimiter struct {
	// mu protects the limiters map.
	mu sync.Mutex
	// limiters maps provider names to their individual rate limiters.
	limiters map[string]*rate.Limiter
	// getConfig returns the current rate limit settings for a given provider.
	getConfig func(provider string) (rate int, burst int)
}

// newRateLimiter creates a [rateLimiter] that reads per-provider rate and burst from the
// provided function on every request, allowing config changes to take effect without restart.
func newRateLimiter(getConfig func(provider string) (rate int, burst int)) *rateLimiter {
	return &rateLimiter{
		limiters:  make(map[string]*rate.Limiter),
		getConfig: getConfig,
	}
}

// Wrap returns middleware that enforces per-provider rate limiting.
func (l *rateLimiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := r.PathValue("provider")
		if !l.getLimiter(provider).Allow() {
			w.Header().Set("Retry-After", "1")
			writeError(w, NewError(http.StatusTooManyRequests, "Rate limit exceeded. Please retry later."))
			return
		}
		next(w, r)
	}
}

// getLimiter returns the [rate.Limiter] for the given provider, creating or
// updating it as needed based on current settings.
func (l *rateLimiter) getLimiter(provider string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Read the current config for this provider.
	rps, burst := l.getConfig(provider)
	if burst < 1 {
		burst = 1
	}

	// Zero or negative value means no limit.
	limit := rate.Limit(rps)
	if rps <= 0 {
		limit = rate.Inf
	}

	lim, ok := l.limiters[provider]
	if !ok {
		// First request for this provider — create a fresh limiter.
		lim = rate.NewLimiter(limit, burst)
		l.limiters[provider] = lim
		return lim
	}

	// Config may have changed since last request — update in place
	// so we keep the current token count instead of resetting it.
	if lim.Limit() != limit {
		lim.SetLimit(limit)
	}
	if lim.Burst() != burst {
		lim.SetBurst(burst)
	}

	return lim
}
