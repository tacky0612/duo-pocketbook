package web_test

// 月次給与 API のテスト。

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDeleteSalaryAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	taro, hanako := acc[0], acc[1]
	token := login(t, srv, taro.LoginID, taro.Password)
	base := srv.URL + "/months/2026-07/salaries/"

	for _, id := range []string{taro.AccountID, hanako.AccountID} {
		if resp, body := doJSON(t, http.MethodPut, base+id, token, map[string]any{"amountYen": 0}); resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
		}
	}

	// 削除すると一覧から消え、精算は 409（未入力）に戻る
	if resp, body := doJSON(t, http.MethodDelete, base+hanako.AccountID, token, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", resp.StatusCode, body)
	}
	resp, body := doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/salaries", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", resp.StatusCode, body)
	}
	var list struct {
		Salaries []struct {
			MemberID string `json:"memberId"`
		} `json:"salaries"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Salaries) != 1 || list.Salaries[0].MemberID != taro.AccountID {
		t.Errorf("salaries = %+v, want taro only", list.Salaries)
	}
	if resp, _ := doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement", token, nil); resp.StatusCode != http.StatusConflict {
		t.Errorf("settlement status = %d, want 409", resp.StatusCode)
	}

	// 未入力のメンバーの削除も 204（冪等）
	if resp, _ := doJSON(t, http.MethodDelete, base+hanako.AccountID, token, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("再削除 status = %d, want 204", resp.StatusCode)
	}
	// 不正な月・不明なメンバーは 400
	if resp, _ := doJSON(t, http.MethodDelete, srv.URL+"/months/2026-13/salaries/"+hanako.AccountID, token, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("不正な月 status = %d, want 400", resp.StatusCode)
	}
	if resp, _ := doJSON(t, http.MethodDelete, base+"nobody", token, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("不明なメンバー status = %d, want 400", resp.StatusCode)
	}
	// 認証なしは 401
	if resp, _ := doJSON(t, http.MethodDelete, base+hanako.AccountID, "", nil); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("未認証 status = %d, want 401", resp.StatusCode)
	}
}
