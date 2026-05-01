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
	limiters map[string]*providerLimiter
	// getConfig returns the current rate limit settings for a given provider.
	getConfig func(provider string) (rate int, burst int)
}

// providerLimiter pairs a [rate.Limiter] with the settings it was created/updated with,
// so we can detect when settings change and update the limiter in place.
type providerLimiter struct {
	limiter *rate.Limiter
	rate    int
	burst   int
}

// newRateLimiter creates a [rateLimiter] that reads per-provider rate and burst from the
// provided function on every request, allowing config changes to take effect without restart.
func newRateLimiter(getConfig func(provider string) (rate int, burst int)) *rateLimiter {
	return &rateLimiter{
		limiters:  make(map[string]*providerLimiter),
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

	pl, ok := l.limiters[provider]
	if !ok {
		// If it's a first request for this provider create a fresh limiter.
		pl = &providerLimiter{
			limiter: rate.NewLimiter(limit, burst),
			rate:    rps,
			burst:   burst,
		}
		l.limiters[provider] = pl
		return pl.limiter
	}

	// Config may have changed since last request — update in place
	// so we keep the current token count instead of resetting it.
	if pl.rate != rps {
		pl.limiter.SetLimit(limit)
		pl.rate = rps
	}
	if pl.burst != burst {
		pl.limiter.SetBurst(burst)
		pl.burst = burst
	}

	return pl.limiter
}
