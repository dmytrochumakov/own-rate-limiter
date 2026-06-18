package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64
	lastRefillTime time.Time
	mutex          sync.Mutex
}

func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:         maxTokens,
		maxTokens:      maxTokens,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

func (r *RateLimiter) refillTokens() {
	now := time.Now()
	duration := now.Sub(r.lastRefillTime).Seconds()
	tokensToAdd := duration * r.refillRate

	r.tokens += tokensToAdd
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}

	r.lastRefillTime = now
}

func (r *RateLimiter) Allow() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.refillTokens()

	if r.tokens >= 1 {
		r.tokens--
		return true
	}

	return false
}

type IPRateLimiter struct {
	limiters map[string]*RateLimiter
	mutex    sync.Mutex
}

func NewIPRateLimiter() *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*RateLimiter),
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *RateLimiter {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	limiter, exists := i.limiters[ip]
	if !exists {
		// allow 3 request per minute
		limiter = NewRateLimiter(3, 0.05)
		i.limiters[ip] = limiter
	}

	return limiter
}

func RateLimitMiddleware(ipRateLimiter *IPRateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		ip, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			http.Error(rw, "Invalid IP", http.StatusInternalServerError)
			return
		}

		limiter := ipRateLimiter.GetLimiter(ip)
		if limiter.Allow() {
			next(rw, req)
		} else {
			http.Error(rw, "Rate Limit Exceeded", http.StatusTooManyRequests)
		}
	}
}

func handleRequest(rw http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(rw, "Request processed successfully at %v\n", time.Now())
}

func main() {
	ipRateLimiter := NewIPRateLimiter()

	mux := http.NewServeMux()
	mux.HandleFunc("/", RateLimitMiddleware(ipRateLimiter, handleRequest))

	fmt.Println("Server starting on :8080")
	http.ListenAndServe(":8080", mux)
}
