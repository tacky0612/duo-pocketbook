package web

import (
	"crypto/subtle"
	"net/http"
	"slices"
)

// CORS はクロスオリジンリクエストを許可するミドルウェア。
// allowedOrigins に "*" が含まれる場合は全オリジンを許可する。
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := slices.Contains(allowedOrigins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowAll || slices.Contains(allowedOrigins, origin)) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Client-Key")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientKeyGuard は事前共有キー（X-Client-Key ヘッダ）が一致しないリクエストを 403 で弾く。
// clientKey が空なら素通し（無効）。ヘルスチェック(/health)と CORS プリフライト(OPTIONS)は対象外。
// 目的は正規クライアント以外（bot等）のリクエストで重い処理・DBアクセスに進む前に遮断すること。
func ClientKeyGuard(clientKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if clientKey == "" {
			return next
		}
		key := []byte(clientKey)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}
			got := []byte(r.Header.Get("X-Client-Key"))
			if subtle.ConstantTimeCompare(got, key) != 1 {
				writeError(w, http.StatusForbidden, "FORBIDDEN", "アクセスが許可されていません")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
