package web_test

// 共有支出の登録から精算までの API テスト。

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestExpenseAndSettlementAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	taro, hanako := acc[0], acc[1]
	taroToken := login(t, srv, taro.LoginID, taro.Password)
	hanakoToken := login(t, srv, hanako.LoginID, hanako.Password)

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/expenses", taroToken, map[string]any{
		"paidBy": taro.AccountID, "amountYen": 20000, "description": "家賃(一部)", "date": "2026-07-01",
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
		"paidBy": hanako.AccountID, "amountYen": 20000, "description": "食費", "date": "2026-07-05",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	resp, body = doJSON(t, http.MethodGet, srv.URL+"/expenses?month=2026-07", taroToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var list struct {
		Expenses []struct {
			ID string `json:"id"`
		} `json:"expenses"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Expenses) != 2 {
		t.Fatalf("len(expenses) = %d, want 2", len(list.Expenses))
	}

	// 給与が揃う前の精算は 409
	resp, _ = doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement", taroToken, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}

	// 給与入力
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/months/2026-07/salaries/"+taro.AccountID, taroToken, map[string]any{
		"amountYen": 100000,
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/months/2026-07/salaries/"+hanako.AccountID, hanakoToken, map[string]any{
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
		settlement.Transfer.From != taro.AccountID || settlement.Transfer.To != hanako.AccountID || settlement.Transfer.AmountYen != 25000 {
		t.Errorf("transfer = %+v, want %s→%s 25000", settlement.Transfer, taro.AccountID, hanako.AccountID)
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
		"paidBy": taro.AccountID, "amountYen": -100, "date": "2026-07-01",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
