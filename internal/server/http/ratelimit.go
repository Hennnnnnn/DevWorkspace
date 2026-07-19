package http

import (
	"net/http"
	"sync"
	"time"
)

type visitor struct {
	lastSeen time.Time
	tokens   int
}

const maxTokens = 20
const refillInterval = time.Second
const refillRate = 2

// rateLimit returns middleware that limits requests per IP using a token bucket.
func rateLimit(next http.HandlerFunc) http.HandlerFunc {
	var mu sync.Mutex
	visitors := make(map[string]*visitor)

	go func() {
		for {
			time.Sleep(refillInterval)
			mu.Lock()
			for ip, v := range visitors {
				v.tokens += refillRate
				if v.tokens > maxTokens {
					v.tokens = maxTokens
				}
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		mu.Lock()
		v, exists := visitors[ip]
		if !exists {
			v = &visitor{tokens: maxTokens}
			visitors[ip] = v
		}
		v.lastSeen = time.Now()
		if v.tokens <= 0 {
			mu.Unlock()
			writeErr(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		v.tokens--
		mu.Unlock()
		next(w, r)
	}
}
