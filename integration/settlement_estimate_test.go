//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// estimateResp は精算レスポンスのうち概算の検証に使うフィールド。
type estimateResp struct {
	Estimated          bool     `json:"estimated"`
	EstimatedMemberIDs []string `json:"estimatedMemberIds"`
	Settled            bool     `json:"settled"`
	Members            []struct {
		ID            string `json:"id"`
		IncomeYen     int64  `json:"incomeYen"`
		DisposableYen int64  `json:"disposableYen"`
	} `json:"members"`
	Transfer *struct {
		From      string `json:"from"`
		To        string `json:"to"`
		AmountYen int64  `json:"amountYen"`
	} `json:"transfer"`
}

// incomeOf は精算レスポンスから指定メンバーの収入を返す。
func (r estimateResp) incomeOf(t *testing.T, id string) int64 {
	t.Helper()
	for _, m := range r.Members {
		if m.ID == id {
			return m.IncomeYen
		}
	}
	t.Fatalf("member %s が精算レスポンスにありません: %+v", id, r.Members)
	return 0
}

// TestSettlementEstimate は estimate=true 指定時に、当月の給与が未入力のメンバーを
// 前月の給与実績で補った概算の精算を返すことを検証する。
// 補うのは給与のみで前月の単発収入は持ち越さないこと、当月の給与が揃えば確定値に
// 切り替わること、概算のままでは精算を完了できないことも確認する。
func TestSettlementEstimate(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	const (
		month     = "2045-06"
		prevMonth = "2045-05"
	)
	url := "/months/" + month + "/settlement?estimate=true"

	getEstimate := func(t *testing.T) estimateResp {
		t.Helper()
		status, body := doJSON(t, http.MethodGet, url, taro, nil)
		if status != http.StatusOK {
			t.Fatalf("settlement(estimate) status = %d, body = %s", status, body)
		}
		var r estimateResp
		if err := json.Unmarshal(body, &r); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return r
	}

	// 当月の共有支出をそれぞれ2万円ずつ登録
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

	// 不正な estimate は 400
	if status, body := doJSON(t, http.MethodGet, "/months/"+month+"/settlement?estimate=maybe", taro, nil); status != http.StatusBadRequest {
		t.Errorf("settlement(estimate=maybe) status = %d, want 400 (body = %s)", status, body)
	}

	// 前月の給与も無ければ概算でも 409
	if status, body := doJSON(t, http.MethodGet, url, taro, nil); status != http.StatusConflict {
		t.Errorf("settlement(前月も未入力) status = %d, want 409 (body = %s)", status, body)
	}

	// 前月: 給与（太郎10万・花子5万）と、花子の前月だけの単発収入3万
	setSalaries(t, taro, taroID, hanako, hanakoID, prevMonth, 100000, 50000)
	if status, body := doJSON(t, http.MethodPost, "/incomes", hanako, map[string]any{
		"memberId": hanakoID, "amountYen": 30000, "description": "前月の臨時収入", "month": prevMonth,
	}); status != http.StatusCreated {
		t.Fatalf("income post status = %d, body = %s", status, body)
	}

	// estimate を指定しなければ従来どおり 409
	if status, body := doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil); status != http.StatusConflict {
		t.Errorf("settlement(estimate なし) status = %d, want 409 (body = %s)", status, body)
	}

	// 両者とも前月の給与で概算: 太郎→花子 25000円（前月の単発収入は含まない）
	r := getEstimate(t)
	if !r.Estimated || len(r.EstimatedMemberIDs) != 2 {
		t.Errorf("estimated = %v, ids = %v, want true with both members", r.Estimated, r.EstimatedMemberIDs)
	}
	if got := r.incomeOf(t, taroID); got != 100000 {
		t.Errorf("taro income = %d, want 100000", got)
	}
	if got := r.incomeOf(t, hanakoID); got != 50000 {
		t.Errorf("hanako income = %d, want 50000（前月の単発収入は持ち越さない）", got)
	}
	if r.Transfer == nil || r.Transfer.From != taroID || r.Transfer.To != hanakoID || r.Transfer.AmountYen != 25000 {
		t.Errorf("transfer = %+v, want taro→hanako 25000", r.Transfer)
	}
	for _, m := range r.Members {
		if m.DisposableYen != 55000 {
			t.Errorf("%s disposable = %d, want 55000", m.ID, m.DisposableYen)
		}
	}

	// 花子だけ当月の給与を入力: 太郎のみ前月実績で補う（双方の純額8万で振込不要）
	if status, body := doJSON(t, http.MethodPut, "/months/"+month+"/salaries/"+hanakoID, hanako, map[string]any{"amountYen": 100000}); status != http.StatusOK {
		t.Fatalf("salary(hanako) status = %d, body = %s", status, body)
	}
	r = getEstimate(t)
	if !r.Estimated || len(r.EstimatedMemberIDs) != 1 || r.EstimatedMemberIDs[0] != taroID {
		t.Errorf("estimated = %v, ids = %v, want true with [taro]", r.Estimated, r.EstimatedMemberIDs)
	}
	if got := r.incomeOf(t, hanakoID); got != 100000 {
		t.Errorf("hanako income = %d, want 100000（当月の入力値）", got)
	}
	if r.Transfer != nil {
		t.Errorf("transfer = %+v, want nil", r.Transfer)
	}

	// 概算のままでは精算を完了できない
	status, body := doJSON(t, http.MethodPut, "/months/"+month+"/settlement/status", taro, map[string]any{"settled": true})
	if status != http.StatusConflict {
		t.Errorf("status put(概算) status = %d, want 409 (body = %s)", status, body)
	}
	var errRes struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errRes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errRes.Error.Code != "INCOME_NOT_READY" {
		t.Errorf("error code = %q, want INCOME_NOT_READY", errRes.Error.Code)
	}

	// 当月の給与が揃えば estimate=true でも確定値（estimated=false・空配列）を返す
	if status, body := doJSON(t, http.MethodPut, "/months/"+month+"/salaries/"+taroID, taro, map[string]any{"amountYen": 200000}); status != http.StatusOK {
		t.Fatalf("salary(taro) status = %d, body = %s", status, body)
	}
	r = getEstimate(t)
	if r.Estimated || r.EstimatedMemberIDs == nil || len(r.EstimatedMemberIDs) != 0 {
		t.Errorf("estimated = %v, ids = %v, want false with empty array", r.Estimated, r.EstimatedMemberIDs)
	}
	// 純額 太郎18万・花子8万 → 太郎→花子 5万円
	if r.Transfer == nil || r.Transfer.From != taroID || r.Transfer.To != hanakoID || r.Transfer.AmountYen != 50000 {
		t.Errorf("transfer = %+v, want taro→hanako 50000", r.Transfer)
	}

	// 確定後は通常の精算（estimate なし）でも同じ結果を返す
	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	var plain estimateResp
	if err := json.Unmarshal(body, &plain); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if plain.Estimated || plain.Transfer == nil || plain.Transfer.AmountYen != 50000 {
		t.Errorf("settlement(estimate なし) = %+v, want estimated=false transfer 50000", plain)
	}
}
