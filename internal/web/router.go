package web

import (
	"net/http"
	"time"
)

// RouterOption はルーター構築のオプション。
type RouterOption struct {
	// StaticDir が空でない場合、APIに一致しないGETリクエストへ静的ファイルを配信する。
	StaticDir string
	// ClientKey が空でない場合、X-Client-Key ヘッダによる事前共有キー検証を有効にする。
	ClientKey string
}

// loginRateLimit はログイン試行のレート制限（IPあたり windowの間に limit 回まで）。
const (
	loginRateLimit  = 10
	loginRateWindow = 5 * time.Minute
)

// NewRouter はAPIルーティングを構築して http.Handler を返す。
func NewRouter(h *Handler, auth *Authenticator, allowedOrigins []string, opt RouterOption) http.Handler {
	mux := http.NewServeMux()

	loginLimiter := newRateLimiter(loginRateLimit, loginRateWindow, nil)

	// 認証不要
	mux.HandleFunc("GET /health", h.Health)
	// ログインは総当り対策として IP 単位のレート制限をかける
	mux.HandleFunc("POST /login", rateLimitByIP(loginLimiter, h.Login))

	// 認証必須
	authed := func(handler http.HandlerFunc) http.Handler {
		return auth.Middleware(handler)
	}
	mux.Handle("GET /members", authed(h.ListMembers))
	mux.Handle("PUT /members/{id}", authed(h.UpdateMember))
	// アカウント（自分の資格情報）
	mux.Handle("GET /account", authed(h.GetAccount))
	mux.Handle("PUT /account/login-id", authed(h.UpdateLoginID))
	mux.Handle("PUT /account/password", authed(h.UpdatePassword))
	mux.Handle("POST /expenses", authed(h.RegisterExpense))
	mux.Handle("GET /expenses", authed(h.ListExpenses))
	mux.Handle("PUT /expenses/{id}", authed(h.UpdateExpense))
	mux.Handle("DELETE /expenses/{id}", authed(h.DeleteExpense))
	mux.Handle("PUT /months/{month}/incomes/{memberId}", authed(h.InputIncome))
	mux.Handle("GET /months/{month}/incomes", authed(h.ListIncomes))
	mux.Handle("GET /months/{month}/settlement", authed(h.GetSettlement))
	mux.Handle("PUT /months/{month}/settlement/status", authed(h.UpdateSettlementStatus))
	mux.Handle("GET /settlements/history", authed(h.GetSettlementHistory))
	mux.Handle("POST /recurring-expenses", authed(h.RegisterRecurringExpense))
	mux.Handle("GET /recurring-expenses", authed(h.ListRecurringExpenses))
	mux.Handle("PUT /recurring-expenses/{id}", authed(h.UpdateRecurringExpense))
	mux.Handle("DELETE /recurring-expenses/{id}", authed(h.DeleteRecurringExpense))
	mux.Handle("POST /direct-transfers", authed(h.RegisterDirectTransfer))
	mux.Handle("GET /direct-transfers", authed(h.ListDirectTransfers))
	mux.Handle("PUT /direct-transfers/{id}", authed(h.UpdateDirectTransfer))
	mux.Handle("DELETE /direct-transfers/{id}", authed(h.DeleteDirectTransfer))
	mux.Handle("GET /settings/weight", authed(h.GetWeight))
	mux.Handle("PUT /settings/weight", authed(h.UpdateWeight))
	mux.Handle("GET /settings/closing-day", authed(h.GetClosingDay))
	mux.Handle("PUT /settings/closing-day", authed(h.UpdateClosingDay))

	// ローカル開発用: フロントエンドの静的配信
	if opt.StaticDir != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(opt.StaticDir)))
	}

	// CORS（最外） → クライアントキー検証 → ルーティング の順。
	// プリフライトは CORS 層で 204 を返すため、キー検証には到達しない。
	return CORS(allowedOrigins)(ClientKeyGuard(opt.ClientKey)(mux))
}
