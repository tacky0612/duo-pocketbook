package web_test

// ログイン・認証（JWT の検証と有効期限）のテスト。

import (
	"net/http"
	"testing"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
	"github.com/tacky0612/duo-pocketbook/internal/web"
)

func TestLogin(t *testing.T) {
	srv, acc := newTestServer(t)

	token := login(t, srv, acc[0].LoginID, acc[0].Password)
	if token == "" {
		t.Fatal("token が空")
	}

	// パスワード誤り
	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{
		"memberId": acc[0].LoginID, "password": "wrong",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}

	// 不明なログインID
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{
		"memberId": "unknown", "password": "taro-pass",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthRequired(t *testing.T) {
	srv, _ := newTestServer(t)

	paths := []struct{ method, path string }{
		{http.MethodGet, "/members"},
		{http.MethodGet, "/account"},
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

	resp, _ := doJSON(t, http.MethodGet, srv.URL+"/members", "invalid-token", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("不正トークン: status = %d, want 401", resp.StatusCode)
	}
}

func TestTokenExpiry(t *testing.T) {
	member := domain.Member{ID: "acct_x", Name: "太郎"}
	couple, err := domain.NewCouple(member, domain.Member{ID: "acct_y", Name: "花子"})
	if err != nil {
		t.Fatalf("NewCouple: %v", err)
	}

	current := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	auth := web.NewAuthenticator("test-secret", time.Hour, couple, func() time.Time { return current })

	token, _, err := auth.IssueToken(member.ID)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := auth.Verify(token); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	current = current.Add(2 * time.Hour)
	if _, err := auth.Verify(token); err == nil {
		t.Fatal("期限切れトークンが有効と判定された")
	}
}
