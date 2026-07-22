package web_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/config"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
	"github.com/tacky0612/duo-pocketbook/internal/infrastructure/memory"
	"github.com/tacky0612/duo-pocketbook/internal/web"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := config.Config{
		Members: [2]config.MemberCredential{
			{Member: domain.Member{ID: "taro", Name: "太郎"}, PasswordPlain: "taro-pass"},
			{Member: domain.Member{ID: "hanako", Name: "花子"}, PasswordPlain: "hanako-pass"},
		},
		JWTSecret:      "test-secret",
		TokenTTL:       time.Hour,
		AllowedOrigins: []string{"*"},
	}
	couple, err := cfg.Couple()
	if err != nil {
		t.Fatalf("Couple: %v", err)
	}

	expenseRepo := memory.NewExpenseRepository()
	incomeRepo := memory.NewIncomeRepository()
	recurringRepo := memory.NewRecurringExpenseRepository()
	settingsRepo := memory.NewSettingsRepository()
	statusRepo := memory.NewSettlementStatusRepository()

	auth := web.NewAuthenticator(cfg, couple, nil)
	handler := web.NewHandler(
		couple,
		auth,
		application.NewExpenseUsecase(couple, expenseRepo, nil),
		application.NewSettlementUsecase(couple, expenseRepo, incomeRepo, recurringRepo, settingsRepo, statusRepo),
		application.NewSettingsUsecase(couple, settingsRepo),
		application.NewRecurringExpenseUsecase(couple, recurringRepo),
	)
	srv := httptest.NewServer(web.NewRouter(handler, auth, cfg.AllowedOrigins, web.RouterOption{}))
	t.Cleanup(srv.Close)
	return srv
}

func doJSON(t *testing.T, method, url, token string, body any) (*http.Response, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, out.Bytes()
}

func login(t *testing.T, srv *httptest.Server, memberID, password string) string {
	t.Helper()
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{
		"memberId": memberID, "password": password,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", resp.StatusCode, body)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out.Token
}

func TestLogin(t *testing.T) {
	srv := newTestServer(t)

	token := login(t, srv, "taro", "taro-pass")
	if token == "" {
		t.Fatal("token が空")
	}

	// パスワード誤り
	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{
		"memberId": "taro", "password": "wrong",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}

	// 不明なメンバー
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{
		"memberId": "unknown", "password": "taro-pass",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthRequired(t *testing.T) {
	srv := newTestServer(t)

	paths := []struct{ method, path string }{
		{http.MethodGet, "/members"},
		{http.MethodPost, "/expenses"},
		{http.MethodGet, "/expenses?month=2026-07"},
		{http.MethodGet, "/months/2026-07/settlement"},
		{http.MethodGet, "/settings/weight"},
	}
	for _, p := range paths {
		resp, _ := doJSON(t, p.method, srv.URL+p.path, "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", p.method, p.path, resp.StatusCode)
		}
	}

	// 不正なトークン
	resp, _ := doJSON(t, http.MethodGet, srv.URL+"/members", "invalid-token", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("不正トークン: status = %d, want 401", resp.StatusCode)
	}
}

func TestExpenseAndSettlementAPI(t *testing.T) {
	srv := newTestServer(t)
	taroToken := login(t, srv, "taro", "taro-pass")
	hanakoToken := login(t, srv, "hanako", "hanako-pass")

	// 支出登録（ユーザー提示の例）
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/expenses", taroToken, map[string]any{
		"paidBy": "taro", "amountYen": 20000, "description": "家賃(一部)", "date": "2026-07-01",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp, body = doJSON(t, http.MethodPost, srv.URL+"/expenses", hanakoToken, map[string]any{
		"paidBy": "hanako", "amountYen": 20000, "description": "食費", "date": "2026-07-05",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	// 一覧
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/expenses?month=2026-07", taroToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var list struct {
		Expenses []struct {
			ID        string `json:"id"`
			AmountYen int64  `json:"amountYen"`
		} `json:"expenses"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Expenses) != 2 {
		t.Fatalf("len(expenses) = %d, want 2", len(list.Expenses))
	}

	// 収入が揃う前の精算は 409
	resp, _ = doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement", taroToken, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}

	// 収入入力
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/months/2026-07/incomes/taro", taroToken, map[string]any{
		"amountYen": 100000,
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/months/2026-07/incomes/hanako", hanakoToken, map[string]any{
		"amountYen": 50000,
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	// 精算: 太郎→花子 25000円
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement", hanakoToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var settlement struct {
		TotalExpenseYen int64 `json:"totalExpenseYen"`
		Transfer        *struct {
			From      string `json:"from"`
			To        string `json:"to"`
			AmountYen int64  `json:"amountYen"`
		} `json:"transfer"`
	}
	if err := json.Unmarshal(body, &settlement); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if settlement.TotalExpenseYen != 40000 {
		t.Errorf("totalExpenseYen = %d, want 40000", settlement.TotalExpenseYen)
	}
	if settlement.Transfer == nil ||
		settlement.Transfer.From != "taro" || settlement.Transfer.To != "hanako" || settlement.Transfer.AmountYen != 25000 {
		t.Errorf("transfer = %+v, want taro→hanako 25000", settlement.Transfer)
	}

	// 支出削除
	resp, _ = doJSON(t, http.MethodDelete, srv.URL+"/expenses/"+created.ID, taroToken, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodDelete, srv.URL+"/expenses/"+created.ID, taroToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("再削除 status = %d, want 404", resp.StatusCode)
	}

	// バリデーションエラー
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/expenses", taroToken, map[string]any{
		"paidBy": "taro", "amountYen": -100, "date": "2026-07-01",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestWeightAPI(t *testing.T) {
	srv := newTestServer(t)
	token := login(t, srv, "taro", "taro-pass")

	// デフォルト 1:1
	resp, body := doJSON(t, http.MethodGet, srv.URL+"/settings/weight", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var weights struct {
		Weights map[string]int64 `json:"weights"`
	}
	if err := json.Unmarshal(body, &weights); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if weights.Weights["taro"] != 1 || weights.Weights["hanako"] != 1 {
		t.Errorf("weights = %v, want 1:1", weights.Weights)
	}

	// 更新
	resp, body = doJSON(t, http.MethodPut, srv.URL+"/settings/weight", token, map[string]any{
		"weights": map[string]int64{"taro": 3, "hanako": 2},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &weights); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if weights.Weights["taro"] != 3 || weights.Weights["hanako"] != 2 {
		t.Errorf("weights = %v, want 3:2", weights.Weights)
	}
}

func TestCORSPreflight(t *testing.T) {
	srv := newTestServer(t)

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/expenses", nil)
	req.Header.Set("Origin", "https://example.github.io")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://example.github.io" {
		t.Errorf("Allow-Origin = %q", got)
	}
}

func TestTokenExpiry(t *testing.T) {
	cfg := config.Config{
		Members: [2]config.MemberCredential{
			{Member: domain.Member{ID: "taro", Name: "太郎"}, PasswordPlain: "pass"},
			{Member: domain.Member{ID: "hanako", Name: "花子"}, PasswordPlain: "pass"},
		},
		JWTSecret: "test-secret",
		TokenTTL:  time.Hour,
	}
	couple, err := cfg.Couple()
	if err != nil {
		t.Fatalf("Couple: %v", err)
	}

	current := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	auth := web.NewAuthenticator(cfg, couple, func() time.Time { return current })

	token, _, _, err := auth.Login("taro", "pass")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := auth.Verify(token); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	// 有効期限を過ぎると検証に失敗する
	current = current.Add(2 * time.Hour)
	if _, err := auth.Verify(token); err == nil {
		t.Fatal("期限切れトークンが有効と判定された")
	}
}
