package web_test

// アカウント API のテスト。

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAccountAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	token := login(t, srv, acc[0].LoginID, acc[0].Password)

	// GET /account
	resp, body := doJSON(t, http.MethodGet, srv.URL+"/account", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var a struct {
		AccountID string `json:"accountId"`
		LoginID   string `json:"loginId"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal(body, &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.AccountID != acc[0].AccountID || a.LoginID != "taro" {
		t.Fatalf("account = %+v", a)
	}

	// ログインID変更
	resp, body = doJSON(t, http.MethodPut, srv.URL+"/account/login-id", token, map[string]string{"loginId": "taro2"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login-id status = %d, body = %s", resp.StatusCode, body)
	}
	// 旧IDはログイン不可・新IDはログイン可
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{"memberId": "taro", "password": "taro-pass"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("旧ID login status = %d, want 401", resp.StatusCode)
	}
	_ = login(t, srv, "taro2", "taro-pass")

	// 他アカウントと重複するIDは 400
	resp, _ = doJSON(t, http.MethodPut, srv.URL+"/account/login-id", token, map[string]string{"loginId": "hanako"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("重複ID status = %d, want 400", resp.StatusCode)
	}

	// パスワード変更（現在PW誤り→400）
	resp, _ = doJSON(t, http.MethodPut, srv.URL+"/account/password", token, map[string]string{
		"currentPassword": "wrong", "newPassword": "newpassword1",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("現在PW誤り status = %d, want 400", resp.StatusCode)
	}
	// 現在PW正しい→204、新PWでログイン可
	resp, _ = doJSON(t, http.MethodPut, srv.URL+"/account/password", token, map[string]string{
		"currentPassword": "taro-pass", "newPassword": "newpassword1",
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PW変更 status = %d, want 204", resp.StatusCode)
	}
	_ = login(t, srv, "taro2", "newpassword1")
}
