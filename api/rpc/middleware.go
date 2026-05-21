package rpc

import (
	"net"
	"net/http"

	lru "github.com/hashicorp/golang-lru/v2"
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
			log.Debugw("connection limit reached, rejecting request",
				"remote", r.RemoteAddr, "max_conns", maxConns,
			)
			http.Error(w, "server busy, try again later", http.StatusServiceUnavailable)
		}
	})
}

// rateLimit returns middleware that enforces per-IP rate limiting.
// Requests exceeding the limit receive 429 Too Many Requests.
func rateLimit(rps, burst, cacheSize int, next http.Handler) http.Handler {
	cache, err := lru.New[string, *rate.Limiter](cacheSize)
	if err != nil {
		panic(err)
	}

	rateL := rate.Limit(rps)
	getLimiter := func(ip string) *rate.Limiter {
		limiter, ok := cache.Get(ip)
		if !ok {
			limiter = rate.NewLimiter(rateL, burst)
			cache.Add(ip, limiter)
		}
		return limiter
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !getLimiter(ip).Allow() {
			// Debug-level to avoid log amplification under sustained rate-limit hits.
			log.Debugw("rate limit exceeded",
				"ip", ip, "rps", rps, "burst", burst,
			)
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
