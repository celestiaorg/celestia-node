package rpc

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// connLimit returns middleware that limits the number of concurrent requests.
// When the limit is reached, new requests receive 503 Service Unavailable.
func connLimit(maxConns int, next http.Handler) http.Handler {
	sem := make(chan struct{}, maxConns)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		default:
			log.Warnw("connection limit reached, rejecting request", "remote", r.RemoteAddr)
			http.Error(w, "server busy, try again later", http.StatusServiceUnavailable)
		}
	})
}

// rateLimit returns middleware that enforces per-IP rate limiting.
// Requests exceeding the limit receive 429 Too Many Requests.
// The background cleanup goroutine exits when ctx is canceled.
func rateLimit(ctx context.Context, rps, burst int, next http.Handler) http.Handler {
	var mu sync.Mutex
	type entry struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	limiters := make(map[string]*entry)
	rateL := rate.Limit(rps)

	// Evict stale entries in the background.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mu.Lock()
				for ip, e := range limiters {
					if time.Since(e.lastSeen) > 10*time.Minute {
						delete(limiters, ip)
					}
				}
				mu.Unlock()
			}
		}
	}()

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		e, ok := limiters[ip]
		if !ok {
			l := rate.NewLimiter(rateL, burst)
			limiters[ip] = &entry{limiter: l, lastSeen: time.Now()}
			return l
		}
		e.lastSeen = time.Now()
		return e.limiter
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !getLimiter(ip).Allow() {
			log.Warnw("rate limit exceeded", "ip", ip)
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractIP returns the IP portion of RemoteAddr (strips port).
func extractIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
