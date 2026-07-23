package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// clientIP はリクエスト元IPを推定する。
// Lambda Function URL は X-Forwarded-For を付与するため、それを優先して先頭のIPを使う。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// maxTrackedIPs は rateLimiter が保持するIP数の上限（メモリ暴走防止）。
const maxTrackedIPs = 10000

// rateLimiter は固定ウィンドウ方式の簡易レートリミッタ（キー＝IP単位）。
// Lambda のウォームインスタンス内でのみ状態を保持する軽量な総当り対策で、
// 予約同時実行数を絞った運用と合わせて機能する（厳密な分散レート制限ではない）。
type rateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string]*hitWindow
}

type hitWindow struct {
	count int
	reset time.Time
}

func newRateLimiter(limit int, window time.Duration, now func() time.Time) *rateLimiter {
	if now == nil {
		now = time.Now
	}
	return &rateLimiter{limit: limit, window: window, now: now, hits: map[string]*hitWindow{}}
}

// Allow は key の今ウィンドウ内の試行が上限未満なら true を返し、カウントを進める。
func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	if len(rl.hits) >= maxTrackedIPs {
		rl.prune(now)
		if len(rl.hits) >= maxTrackedIPs {
			rl.hits = map[string]*hitWindow{}
		}
	}

	w, ok := rl.hits[key]
	if !ok || now.After(w.reset) {
		rl.hits[key] = &hitWindow{count: 1, reset: now.Add(rl.window)}
		return true
	}
	if w.count >= rl.limit {
		return false
	}
	w.count++
	return true
}

// prune は期限切れウィンドウを削除する（ロック保持中に呼ぶ）。
func (rl *rateLimiter) prune(now time.Time) {
	for k, w := range rl.hits {
		if now.After(w.reset) {
			delete(rl.hits, k)
		}
	}
}

// rateLimitByIP は IP 単位のレート制限を行うミドルウェア。上限超過時は 429 を返す。
func rateLimitByIP(rl *rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(clientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "リクエストが多すぎます。しばらくしてからお試しください。")
			return
		}
		next(w, r)
	}
}
