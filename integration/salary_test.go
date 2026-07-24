//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestSalaryAndSettlement は給与の入力・一覧・精算の可否（未入力なら409）と、
// 給与＋共有支出から算出する基本の精算（比重1:1）を検証する。
func TestSalaryAndSettlement(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	const month = "2032-02"

	// 給与が揃う前の精算は 409
	if status, _ := doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil); status != http.StatusConflict {
		t.Errorf("settlement(給与未入力) status = %d, want 409", status)
	}

	// 共有支出をそれぞれ2万円ずつ登録
	for _, e := range []struct {
		token, paidBy, desc string
	}{
		{taro, taroID, "家賃(一部)"},
		{hanako, hanakoID, "食費"},
	} {
		if status, body := doJSON(t, http.MethodPost, "/expenses", e.token, map[string]any{
			"paidBy": e.paidBy, "amountYen": 20000, "description": e.desc, "date": month + "-15",
		}); status != http.StatusCreated {
			t.Fatalf("expense post status = %d, body = %s", status, body)
		}
	}

	// 給与入力（PUT レスポンスも検証）
	status, body := doJSON(t, http.MethodPut, "/months/"+month+"/salaries/"+taroID, taro, map[string]any{"amountYen": 100000})
	if status != http.StatusOK {
		t.Fatalf("salary put status = %d, body = %s", status, body)
	}
	var putRes struct {
		Month  string `json:"month"`
		Salary struct {
			MemberID  string `json:"memberId"`
			AmountYen int64  `json:"amountYen"`
		} `json:"salary"`
	}
	if err := json.Unmarshal(body, &putRes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if putRes.Month != month || putRes.Salary.MemberID != taroID || putRes.Salary.AmountYen != 100000 {
		t.Errorf("salary put レスポンス = %+v", putRes)
	}
	if status, _ := doJSON(t, http.MethodPut, "/months/"+month+"/salaries/"+hanakoID, hanako, map[string]any{"amountYen": 50000}); status != http.StatusOK {
		t.Fatalf("salary(hanako) put status = %d", status)
	}

	// 給与一覧は2件
	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/salaries", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("salary list status = %d, body = %s", status, body)
	}
	var listRes struct {
		Salaries []struct {
			MemberID  string `json:"memberId"`
			AmountYen int64  `json:"amountYen"`
		} `json:"salaries"`
	}
	if err := json.Unmarshal(body, &listRes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(listRes.Salaries) != 2 {
		t.Fatalf("salaries = %d, want 2 (body = %s)", len(listRes.Salaries), body)
	}

	// 精算: 比重1:1 → 太郎が花子に25000円、可処分は双方55000円
	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	var s struct {
		TotalExpenseYen int64 `json:"totalExpenseYen"`
		Members         []struct {
			ID            string `json:"id"`
			DisposableYen int64  `json:"disposableYen"`
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
	if s.TotalExpenseYen != 40000 {
		t.Errorf("totalExpenseYen = %d, want 40000", s.TotalExpenseYen)
	}
	if s.Transfer == nil || s.Transfer.From != taroID || s.Transfer.To != hanakoID || s.Transfer.AmountYen != 25000 {
		t.Errorf("transfer = %+v, want taro→hanako 25000", s.Transfer)
	}
	for _, m := range s.Members {
		if m.DisposableYen != 55000 {
			t.Errorf("%s disposable = %d, want 55000", m.ID, m.DisposableYen)
		}
	}
}
