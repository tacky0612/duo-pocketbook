//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAuthAndMembers は認証（成功・失敗・トークンなし）とメンバー一覧を検証する。
func TestAuthAndMembers(t *testing.T) {
	waitForHealthy(t)
	taro, _ := auth(t, "taro", "taro-password")

	// 誤ったパスワードは 401
	if status, _ := doJSON(t, http.MethodPost, "/login", "", map[string]string{
		"memberId": "taro", "password": "wrong",
	}); status != http.StatusUnauthorized {
		t.Errorf("誤パスワード: status = %d, want 401", status)
	}

	// トークンなしのアクセスは 401
	if status, _ := doJSON(t, http.MethodGet, "/members", "", nil); status != http.StatusUnauthorized {
		t.Errorf("トークンなし: status = %d, want 401", status)
	}

	// メンバー一覧は2人
	status, body := doJSON(t, http.MethodGet, "/members", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("members status = %d, body = %s", status, body)
	}
	var res struct {
		Members []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"members"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(res.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(res.Members))
	}
	for _, m := range res.Members {
		if m.ID == "" || m.Name == "" {
			t.Errorf("member に空のフィールド: %+v", m)
		}
	}
}

// TestAccount は認証中アカウント情報の取得と、ログインID・パスワードの変更を検証する。
// 変更は他テストのログインに影響するため、テスト内で必ず元に戻す。
// JWT の subject は不変の AccountID のため、変更後も同じトークンで戻せる。
func TestAccount(t *testing.T) {
	waitForHealthy(t)
	hanako, hanakoID := auth(t, "hanako", "hanako-password")

	// GET /account
	status, body := doJSON(t, http.MethodGet, "/account", hanako, nil)
	if status != http.StatusOK {
		t.Fatalf("account get status = %d, body = %s", status, body)
	}
	var acc struct {
		AccountID string `json:"accountId"`
		LoginID   string `json:"loginId"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal(body, &acc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if acc.AccountID != hanakoID || acc.LoginID != "hanako" || acc.Name == "" {
		t.Errorf("account = %+v, want accountId=%s loginId=hanako name!=空", acc, hanakoID)
	}

	// --- パスワード変更（元に戻す） ---
	const origPw, newPw = "hanako-password", "hanako-password-2"
	changePw := func(cur, next string) int {
		s, _ := doJSON(t, http.MethodPut, "/account/password", hanako, map[string]any{
			"currentPassword": cur, "newPassword": next,
		})
		return s
	}
	if s := changePw(origPw, newPw); s != http.StatusNoContent {
		t.Fatalf("password 変更 status = %d, want 204", s)
	}
	defer func() {
		if s := changePw(newPw, origPw); s != http.StatusNoContent {
			t.Errorf("password 復元 status = %d, want 204", s)
		}
	}()
	// 新パスワードでログインできる
	if s, _ := doJSON(t, http.MethodPost, "/login", "", map[string]string{
		"memberId": "hanako", "password": newPw,
	}); s != http.StatusOK {
		t.Errorf("新パスワードでのログイン status = %d, want 200", s)
	}
	// 誤った現在パスワードでの変更は 400
	if s := changePw("wrong-current", "whatever-8"); s != http.StatusBadRequest {
		t.Errorf("誤った現在パスワード status = %d, want 400", s)
	}

	// --- ログインID変更（元に戻す） ---
	changeLoginID := func(id string) int {
		s, _ := doJSON(t, http.MethodPut, "/account/login-id", hanako, map[string]any{"loginId": id})
		return s
	}
	if s := changeLoginID("hanako2"); s != http.StatusOK {
		t.Fatalf("loginId 変更 status = %d, want 200", s)
	}
	defer func() {
		if s := changeLoginID("hanako"); s != http.StatusOK {
			t.Errorf("loginId 復元 status = %d, want 200", s)
		}
	}()
	// 新しいログインIDでログインできる（パスワードは復元前なので newPw）
	if s, _ := doJSON(t, http.MethodPost, "/login", "", map[string]string{
		"memberId": "hanako2", "password": newPw,
	}); s != http.StatusOK {
		t.Errorf("新ログインIDでのログイン status = %d, want 200", s)
	}
}

// TestMemberProfile はメンバーの表示名・カラーの上書き更新を検証する。
// 表示は全体設定のため、元の値に戻して他テストへの影響を避ける。
func TestMemberProfile(t *testing.T) {
	waitForHealthy(t)
	taro, taroID := auth(t, "taro", "taro-password")

	// 元の表示名・カラーを取得
	getMember := func(id string) (name, color string) {
		_, body := doJSON(t, http.MethodGet, "/members", taro, nil)
		var res struct {
			Members []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"members"`
		}
		if err := json.Unmarshal(body, &res); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		for _, m := range res.Members {
			if m.ID == id {
				return m.Name, m.Color
			}
		}
		t.Fatalf("member %s が見つかりません", id)
		return "", ""
	}
	origName, origColor := getMember(taroID)

	// 更新
	status, body := doJSON(t, http.MethodPut, "/members/"+taroID, taro, map[string]any{
		"name": "テスト表示名", "color": "#123456",
	})
	if status != http.StatusOK {
		t.Fatalf("member update status = %d, body = %s", status, body)
	}
	if name, color := getMember(taroID); name != "テスト表示名" || color != "#123456" {
		t.Errorf("更新後 = (%s, %s), want (テスト表示名, #123456)", name, color)
	}

	// 復元（元のカラーが空の場合はカラー更新をスキップ）
	revert := map[string]any{"name": origName}
	if origColor != "" {
		revert["color"] = origColor
	}
	if status, _ := doJSON(t, http.MethodPut, "/members/"+taroID, taro, revert); status != http.StatusOK {
		t.Errorf("member 復元 status = %d, want 200", status)
	}
}
