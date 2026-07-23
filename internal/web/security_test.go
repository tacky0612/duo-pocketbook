package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientKeyGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	guard := ClientKeyGuard("secret-key")(ok)

	tests := []struct {
		name   string
		method string
		path   string
		key    string
		want   int
	}{
		{"正しいキー", http.MethodGet, "/members", "secret-key", http.StatusOK},
		{"キー不一致", http.MethodGet, "/members", "wrong", http.StatusForbidden},
		{"キーなし", http.MethodGet, "/members", "", http.StatusForbidden},
		{"health は対象外", http.MethodGet, "/health", "", http.StatusOK},
		{"プリフライトは対象外", http.MethodOptions, "/members", "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.key != "" {
				req.Header.Set("X-Client-Key", tt.key)
			}
			rec := httptest.NewRecorder()
			guard.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestClientKeyGuardDisabledWhenEmpty(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	// キー未設定なら素通し
	guard := ClientKeyGuard("")(next)
	req := httptest.NewRequest(http.MethodGet, "/members", nil)
	guard.ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Fatal("キー未設定時は素通しされるべき")
	}
}

func TestRateLimiterAllow(t *testing.T) {
	now := time.Unix(0, 0)
	rl := newRateLimiter(3, time.Minute, func() time.Time { return now })

	// 上限まで許可
	for i := 0; i < 3; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("%d回目は許可されるべき", i+1)
		}
	}
	// 上限超過は拒否
	if rl.Allow("1.2.3.4") {
		t.Fatal("上限超過は拒否されるべき")
	}
	// 別IPは独立
	if !rl.Allow("5.6.7.8") {
		t.Fatal("別IPは独立して許可されるべき")
	}
	// ウィンドウ経過で回復
	now = now.Add(time.Minute + time.Second)
	if !rl.Allow("1.2.3.4") {
		t.Fatal("ウィンドウ経過後は再び許可されるべき")
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name string
		xff  string
		addr string
		want string
	}{
		{"XFF 単一", "203.0.113.7", "10.0.0.1:1234", "203.0.113.7"},
		{"XFF 複数は先頭", "203.0.113.7, 70.41.3.18", "10.0.0.1:1234", "203.0.113.7"},
		{"XFFなしはRemoteAddr", "", "192.0.2.5:5555", "192.0.2.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/login", nil)
			req.RemoteAddr = tt.addr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := clientIP(req); got != tt.want {
				t.Fatalf("clientIP = %q, want %q", got, tt.want)
			}
		})
	}
}
