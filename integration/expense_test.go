//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

type expenseResp struct {
	ID          string `json:"id"`
	PaidBy      string `json:"paidBy"`
	AmountYen   int64  `json:"amountYen"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Month       string `json:"month"`
}

func listExpenses(t *testing.T, token, month string) []expenseResp {
	t.Helper()
	status, body := doJSON(t, http.MethodGet, "/expenses?month="+month, token, nil)
	if status != http.StatusOK {
		t.Fatalf("expense list(%s) status = %d, body = %s", month, status, body)
	}
	var res struct {
		Expenses []expenseResp `json:"expenses"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return res.Expenses
}

// TestExpenseCRUD は共有支出の登録・一覧・更新（同月・別月移動）・削除を検証する。
func TestExpenseCRUD(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	const month = "2031-03"

	// 登録
	status, body := doJSON(t, http.MethodPost, "/expenses", taro, map[string]any{
		"paidBy": taroID, "amountYen": 20000, "description": "家賃", "date": month + "-15",
	})
	if status != http.StatusCreated {
		t.Fatalf("expense post status = %d, body = %s", status, body)
	}
	var created expenseResp
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Month != month || created.AmountYen != 20000 {
		t.Fatalf("登録結果 = %+v", created)
	}

	// 一覧に現れる
	if list := listExpenses(t, hanako, month); len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("一覧 = %+v, want 1件 (%s)", list, created.ID)
	}

	// 同月内の更新（IDは維持、内容が変わる）
	status, body = doJSON(t, http.MethodPut, "/expenses/"+created.ID, taro, map[string]any{
		"paidBy": hanakoID, "amountYen": 4500, "description": "家賃(訂正)", "date": month + "-20",
	})
	if status != http.StatusOK {
		t.Fatalf("expense update status = %d, body = %s", status, body)
	}
	var updated expenseResp
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.ID != created.ID || updated.PaidBy != hanakoID || updated.AmountYen != 4500 || updated.Description != "家賃(訂正)" {
		t.Errorf("同月更新 = %+v, want ID維持/paidBy=hanako/4500/家賃(訂正)", updated)
	}

	// 別月への更新（旧月から消え、新月に移動）
	const nextMonth = "2031-04"
	status, body = doJSON(t, http.MethodPut, "/expenses/"+created.ID, taro, map[string]any{
		"paidBy": hanakoID, "amountYen": 4500, "description": "家賃(訂正)", "date": nextMonth + "-03",
	})
	if status != http.StatusOK {
		t.Fatalf("expense 別月更新 status = %d, body = %s", status, body)
	}
	var moved expenseResp
	if err := json.Unmarshal(body, &moved); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if moved.Month != nextMonth {
		t.Errorf("移動後の月 = %s, want %s", moved.Month, nextMonth)
	}
	if list := listExpenses(t, taro, month); len(list) != 0 {
		t.Errorf("旧月の一覧 = %d件, want 0", len(list))
	}
	if list := listExpenses(t, taro, nextMonth); len(list) != 1 {
		t.Errorf("新月の一覧 = %d件, want 1", len(list))
	}

	// 削除と再削除（404）
	if status, _ := doJSON(t, http.MethodDelete, "/expenses/"+moved.ID, taro, nil); status != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", status)
	}
	if status, _ := doJSON(t, http.MethodDelete, "/expenses/"+moved.ID, taro, nil); status != http.StatusNotFound {
		t.Errorf("再削除 status = %d, want 404", status)
	}
}
