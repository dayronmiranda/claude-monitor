package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	apierrors "claude-monitor/pkg/errors"
	"claude-monitor/pkg/logger"
	"claude-monitor/pkg/metrics"
)

// RateLimiter implementa rate limiting por IP
type RateLimiter struct {
	limiters map[string]*clientLimiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	cleanup  time.Duration
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter crea un nuevo rate limiter
// rps: requests per second permitidos
// burst: número de requests en ráfaga permitidos
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*clientLimiter),
		rate:     rate.Limit(rps),
		burst:    burst,
		cleanup:  3 * time.Minute,
	}

	// Goroutine para limpiar limiters antiguos
	go rl.cleanupLoop()

	return rl
}

// getLimiter obtiene o crea un limiter para una IP
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if client, exists := rl.limiters[ip]; exists {
		client.lastSeen = time.Now()
		return client.limiter
	}

	limiter := rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[ip] = &clientLimiter{
		limiter:  limiter,
		lastSeen: time.Now(),
	}

	return limiter
}

// cleanupLoop limpia limiters inactivos
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for ip, client := range rl.limiters {
			if time.Since(client.lastSeen) > rl.cleanup {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware retorna el middleware HTTP
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		limiter := rl.getLimiter(ip)

		if !limiter.Allow() {
			logger.Warn("Rate limit exceeded",
				"ip", ip,
				"path", r.URL.Path,
				"method", r.Method)

			// Record rate limit hit metric
			metrics.RecordRateLimitHit(ip)

			w.Header().Set("Retry-After", "1")
			apierrors.WriteError(w, apierrors.TooManyRequests("rate limit exceeded"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getClientIP obtiene la IP real del cliente
func getClientIP(r *http.Request) string {
	// Verificar headers de proxy
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Tomar la primera IP (cliente original)
		if ip, _, err := net.SplitHostPort(xff); err == nil {
			return ip
		}
		return xff
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback a RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// Stats retorna estadísticas del rate limiter
func (rl *RateLimiter) Stats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"active_limiters": len(rl.limiters),
		"rate_limit":      float64(rl.rate),
		"burst_limit":     rl.burst,
	}
}
