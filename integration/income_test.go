//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestAdditionalIncome は給与とは別の追加収入の登録・一覧・更新・削除と、精算への合算を検証する。
func TestAdditionalIncome(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	const month = "2034-05"

	// 給与を両者8万ずつ（精算不要の状態）
	setSalaries(t, taro, taroID, hanako, hanakoID, month, 80000, 80000)

	// 継続の追加収入（太郎・副業2万）を登録
	status, body := doJSON(t, http.MethodPost, "/incomes", taro, map[string]any{
		"memberId": taroID, "amountYen": 20000, "description": "副業", "month": "",
	})
	if status != http.StatusCreated {
		t.Fatalf("income post status = %d, body = %s", status, body)
	}
	var created struct {
		ID        string `json:"id"`
		MemberID  string `json:"memberId"`
		Recurring bool   `json:"recurring"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !created.Recurring || created.MemberID != taroID {
		t.Fatalf("登録結果 = %+v, want recurring=true memberId=taro", created)
	}

	// 一覧に1件
	status, body = doJSON(t, http.MethodGet, "/incomes?month="+month, taro, nil)
	if status != http.StatusOK {
		t.Fatalf("income list status = %d, body = %s", status, body)
	}
	var list struct {
		Incomes []struct {
			ID string `json:"id"`
		} `json:"incomes"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Incomes) != 1 {
		t.Fatalf("incomes = %d, want 1 (body = %s)", len(list.Incomes), body)
	}

	// 更新（金額を3万へ。継続/単発の別は維持される）
	status, body = doJSON(t, http.MethodPut, "/incomes/"+created.ID, taro, map[string]any{
		"memberId": taroID, "amountYen": 30000, "description": "副業(増)",
	})
	if status != http.StatusOK {
		t.Fatalf("income update status = %d, body = %s", status, body)
	}
	var updated struct {
		AmountYen   int64  `json:"amountYen"`
		Description string `json:"description"`
		Recurring   bool   `json:"recurring"`
	}
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.AmountYen != 30000 || updated.Description != "副業(増)" || !updated.Recurring {
		t.Errorf("更新結果 = %+v, want 30000/副業(増)/recurring", updated)
	}

	// 精算に反映: 太郎 80000+30000=110000 対 花子 80000 → 太郎→花子 15000
	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	var s struct {
		Members []struct {
			ID        string `json:"id"`
			IncomeYen int64  `json:"incomeYen"`
		} `json:"members"`
		Transfer *struct {
			From      string `json:"from"`
			To        string `json:"to"`
			AmountYen int64  `json:"amountYen"`
		} `json:"transfer"`
	}
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, m := range s.Members {
		if m.ID == taroID && m.IncomeYen != 110000 {
			t.Errorf("taro incomeYen = %d, want 110000（給与8万＋追加3万）", m.IncomeYen)
		}
	}
	if s.Transfer == nil || s.Transfer.From != taroID || s.Transfer.To != hanakoID || s.Transfer.AmountYen != 15000 {
		t.Errorf("transfer = %+v, want taro→hanako 15000", s.Transfer)
	}

	// 削除と再削除（404）
	if status, _ := doJSON(t, http.MethodDelete, "/incomes/"+created.ID, taro, nil); status != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", status)
	}
	if status, _ := doJSON(t, http.MethodDelete, "/incomes/"+created.ID, taro, nil); status != http.StatusNotFound {
		t.Errorf("再削除 status = %d, want 404", status)
	}
}
