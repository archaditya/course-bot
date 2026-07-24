package http

import (
    "context"
    "net/http"
    "sync"
    "time"
)

type RateLimiter struct {
    limits map[string]*userLimiter
    mu     sync.RWMutex
    rate   int           // requests per minute
    burst  int           // burst size
    cleanupInterval time.Duration
    stopCleanup chan struct{}
}

type userLimiter struct {
    tokens      int
    lastRefill  time.Time
    lastUsed    time.Time
    mu          sync.Mutex
}

func NewRateLimiter(rate, burst int) *RateLimiter {
    rl := &RateLimiter{
        limits: make(map[string]*userLimiter),
        rate:   rate,
        burst:  burst,
        cleanupInterval: 5 * time.Minute,
        stopCleanup: make(chan struct{}),
    }
    go rl.cleanup()
    return rl
}

func (rl *RateLimiter) Stop() {
    close(rl.stopCleanup)
}

func (rl *RateLimiter) cleanup() {
    ticker := time.NewTicker(rl.cleanupInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            rl.mu.Lock()
            now := time.Now()
            for userID, limiter := range rl.limits {
                limiter.mu.Lock()
                if now.Sub(limiter.lastUsed) > rl.cleanupInterval {
                    delete(rl.limits, userID)
                }
                limiter.mu.Unlock()
            }
            rl.mu.Unlock()
        case <-rl.stopCleanup:
            return
        }
    }
}

func (rl *RateLimiter) Allow(userID string) bool {
    rl.mu.RLock()
    limiter, exists := rl.limits[userID]
    if !exists {
        rl.mu.RUnlock()
        rl.mu.Lock()
        limiter = &userLimiter{
            tokens:     rl.burst,
            lastRefill: time.Now(),
            lastUsed:   time.Now(),
        }
        rl.limits[userID] = limiter
        rl.mu.Unlock()
    } else {
        rl.mu.RUnlock()
    }
    
    limiter.mu.Lock()
    defer limiter.mu.Unlock()
    
    now := time.Now()
    limiter.lastUsed = now
    
    // Calculate tokens to add based on elapsed time
    elapsed := now.Sub(limiter.lastRefill).Seconds()
    tokensToAdd := int(elapsed * float64(rl.rate) / 60.0) // rate is per minute
    
    if tokensToAdd > 0 {
        limiter.tokens += tokensToAdd
        if limiter.tokens > rl.burst {
            limiter.tokens = rl.burst
        }
        limiter.lastRefill = now
    }
    
    if limiter.tokens > 0 {
        limiter.tokens--
        return true
    }
    
    return false
}

func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, ok := ClaimsFromContext(r.Context())
            if !ok {
                WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Missing access token.")
                return
            }
            
            if !limiter.Allow(claims.UserID) {
                WriteError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests. Please try again later.")
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}