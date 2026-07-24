//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestRecurringExpense は固定費の登録・一覧・更新・削除と、精算への反映を検証する。
// 固定費は全月の精算へ加算されるグローバル資源のため、テスト内で必ず削除して後始末する。
func TestRecurringExpense(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	const month = "2036-06"

	// 登録
	status, body := doJSON(t, http.MethodPost, "/recurring-expenses", taro, map[string]any{
		"paidBy": taroID, "amountYen": 50000, "description": "家賃",
	})
	if status != http.StatusCreated {
		t.Fatalf("recurring post status = %d, body = %s", status, body)
	}
	var created struct {
		ID        string `json:"id"`
		AmountYen int64  `json:"amountYen"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// 失敗時もリークしないよう削除を予約
	deleted := false
	defer func() {
		if deleted {
			return
		}
		if status, _ := doJSON(t, http.MethodDelete, "/recurring-expenses/"+created.ID, taro, nil); status != http.StatusNoContent {
			t.Errorf("cleanup delete status = %d, want 204", status)
		}
	}()

	// 一覧に現れる
	status, body = doJSON(t, http.MethodGet, "/recurring-expenses", hanako, nil)
	if status != http.StatusOK {
		t.Fatalf("recurring list status = %d, body = %s", status, body)
	}
	var list struct {
		RecurringExpenses []struct {
			ID string `json:"id"`
		} `json:"recurringExpenses"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	found := false
	for _, r := range list.RecurringExpenses {
		if r.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("一覧に登録した固定費が見つかりません: %s (body = %s)", created.ID, body)
	}

	// 更新（支払者・金額・内容）
	status, body = doJSON(t, http.MethodPut, "/recurring-expenses/"+created.ID, taro, map[string]any{
		"paidBy": hanakoID, "amountYen": 60000, "description": "家賃(更新)",
	})
	if status != http.StatusOK {
		t.Fatalf("recurring update status = %d, body = %s", status, body)
	}
	var updated struct {
		PaidBy      string `json:"paidBy"`
		AmountYen   int64  `json:"amountYen"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.PaidBy != hanakoID || updated.AmountYen != 60000 || updated.Description != "家賃(更新)" {
		t.Errorf("更新結果 = %+v", updated)
	}

	// 精算に固定費が共有支出として加算される（当月に通常支出なし → 合計は固定費のみ）
	setSalaries(t, taro, taroID, hanako, hanakoID, month, 100000, 100000)
	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	var s struct {
		TotalExpenseYen int64 `json:"totalExpenseYen"`
		Members         []struct {
			ID             string `json:"id"`
			PaidExpenseYen int64  `json:"paidExpenseYen"`
		} `json:"members"`
	}
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.TotalExpenseYen != 60000 {
		t.Errorf("totalExpenseYen = %d, want 60000（固定費）", s.TotalExpenseYen)
	}
	for _, m := range s.Members {
		if m.ID == hanakoID && m.PaidExpenseYen != 60000 {
			t.Errorf("hanako paidExpense = %d, want 60000", m.PaidExpenseYen)
		}
	}

	// 削除
	if status, _ := doJSON(t, http.MethodDelete, "/recurring-expenses/"+created.ID, taro, nil); status != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", status)
	}
	deleted = true

	// 削除後は精算の合計支出が0に戻る
	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement(削除後) status = %d, body = %s", status, body)
	}
	s.TotalExpenseYen = -1
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.TotalExpenseYen != 0 {
		t.Errorf("削除後 totalExpenseYen = %d, want 0", s.TotalExpenseYen)
	}

	// 再削除は 404
	if status, _ := doJSON(t, http.MethodDelete, "/recurring-expenses/"+created.ID, taro, nil); status != http.StatusNotFound {
		t.Errorf("再削除 status = %d, want 404", status)
	}
}
