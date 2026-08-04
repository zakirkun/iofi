package api

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// requireToken guards every /api route. The API can hire agents, dispatch runs
// with filesystem access, and read the whole shared memory — it is not a
// read-only dashboard, and it must not be open.
//
// EventSource cannot set headers, so SSE passes the token as a query parameter.
func requireToken(token string) func(http.Handler) http.Handler {
	want := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}
			got := r.URL.Query().Get("token")
			if h := r.Header.Get("Authorization"); h != "" {
				if t, ok := strings.CutPrefix(h, "Bearer "); ok {
					got = strings.TrimSpace(t)
				}
			}
			if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="iofi"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimit caps requests per client IP. It bounds the damage a runaway script
// or a leaked token can do before anyone notices.
func rateLimit(perMinute int) func(http.Handler) http.Handler {
	if perMinute <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	var (
		mu      sync.Mutex
		buckets = map[string]*visitor{}
	)
	// Evict idle buckets so a long-running process does not accumulate one per
	// address it has ever seen.
	go func() {
		for range time.Tick(5 * time.Minute) {
			mu.Lock()
			for ip, v := range buckets {
				if time.Since(v.seen) > 10*time.Minute {
					delete(buckets, ip)
				}
			}
			mu.Unlock()
		}
	}()

	limit := rate.Limit(float64(perMinute) / 60.0)
	burst := perMinute / 4
	if burst < 10 {
		burst = 10
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			mu.Lock()
			v, ok := buckets[ip]
			if !ok {
				v = &visitor{lim: rate.NewLimiter(limit, burst)}
				buckets[ip] = v
			}
			v.seen = time.Now()
			allowed := v.lim.Allow()
			mu.Unlock()

			if !allowed {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type visitor struct {
	lim  *rate.Limiter
	seen time.Time
}
