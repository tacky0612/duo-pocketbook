package web_test

// 月次精算 API（概算）のテスト。

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSettlementEstimateAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	taro, hanako := acc[0], acc[1]
	token := login(t, srv, taro.LoginID, taro.Password)

	for _, e := range []map[string]any{
		{"paidBy": taro.AccountID, "amountYen": 20000, "description": "家賃(一部)", "date": "2026-07-01"},
		{"paidBy": hanako.AccountID, "amountYen": 20000, "description": "食費", "date": "2026-07-05"},
	} {
		if resp, body := doJSON(t, http.MethodPost, srv.URL+"/expenses", token, e); resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
		}
	}

	// 不正な estimate は 400
	if resp, _ := doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement?estimate=maybe", token, nil); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	// 前月の給与も無ければ概算でも 409
	if resp, _ := doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement?estimate=true", token, nil); resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}

	// 前月の給与を入力する
	for id, amount := range map[string]int{taro.AccountID: 100000, hanako.AccountID: 50000} {
		if resp, body := doJSON(t, http.MethodPut, srv.URL+"/months/2026-06/salaries/"+id, token, map[string]any{"amountYen": amount}); resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
		}
	}

	// estimate を指定しなければ従来どおり 409
	if resp, _ := doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement", token, nil); resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}

	type settlementBody struct {
		Estimated          bool     `json:"estimated"`
		EstimatedMemberIDs []string `json:"estimatedMemberIds"`
		Transfer           *struct {
			From      string `json:"from"`
			To        string `json:"to"`
			AmountYen int64  `json:"amountYen"`
		} `json:"transfer"`
	}
	get := func(url string) settlementBody {
		t.Helper()
		resp, body := doJSON(t, http.MethodGet, url, token, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
		}
		var s settlementBody
		if err := json.Unmarshal(body, &s); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return s
	}

	// 前月実績で概算: 太郎→花子 25000円
	s := get(srv.URL + "/months/2026-07/settlement?estimate=true")
	if !s.Estimated || len(s.EstimatedMemberIDs) != 2 {
		t.Errorf("estimated = %v, ids = %v, want true with both members", s.Estimated, s.EstimatedMemberIDs)
	}
	if s.Transfer == nil || s.Transfer.From != taro.AccountID || s.Transfer.To != hanako.AccountID || s.Transfer.AmountYen != 25000 {
		t.Errorf("transfer = %+v, want %s→%s 25000", s.Transfer, taro.AccountID, hanako.AccountID)
	}

	// 当月が揃えば estimate=true でも確定値（estimated=false）を返す
	for id, amount := range map[string]int{taro.AccountID: 100000, hanako.AccountID: 100000} {
		if resp, body := doJSON(t, http.MethodPut, srv.URL+"/months/2026-07/salaries/"+id, token, map[string]any{"amountYen": amount}); resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
		}
	}
	s = get(srv.URL + "/months/2026-07/settlement?estimate=true")
	if s.Estimated || s.EstimatedMemberIDs == nil || len(s.EstimatedMemberIDs) != 0 {
		t.Errorf("estimated = %v, ids = %v, want false with empty array", s.Estimated, s.EstimatedMemberIDs)
	}
	if s.Transfer != nil {
		t.Errorf("transfer = %+v, want nil", s.Transfer)
	}
}
