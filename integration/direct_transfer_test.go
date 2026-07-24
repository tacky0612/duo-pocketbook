//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestDirectTransfer は立替精算の登録（継続・単発）・一覧・更新・削除と、
// 精算（比重按分とは別枠で振込額へ純額加算）への反映を検証する。
func TestDirectTransfer(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	const month = "2040-07"

	// 継続: 太郎 → 花子 5000（毎月）
	status, body := doJSON(t, http.MethodPost, "/direct-transfers", taro, map[string]any{
		"from": taroID, "amountYen": 5000, "description": "毎月のお小遣い",
	})
	if status != http.StatusCreated {
		t.Fatalf("direct(継続) status = %d, body = %s", status, body)
	}
	var recurring struct {
		ID        string `json:"id"`
		To        string `json:"to"`
		Recurring bool   `json:"recurring"`
	}
	if err := json.Unmarshal(body, &recurring); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !recurring.Recurring || recurring.To != hanakoID {
		t.Fatalf("継続登録 = %+v, want recurring=true to=hanako", recurring)
	}

	// 単発: 花子 → 太郎 2000（2040-07 のみ）
	status, body = doJSON(t, http.MethodPost, "/direct-transfers", hanako, map[string]any{
		"from": hanakoID, "amountYen": 2000, "description": "立替の返済", "month": month,
	})
	if status != http.StatusCreated {
		t.Fatalf("direct(単発) status = %d, body = %s", status, body)
	}
	var oneOff struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &oneOff); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// 一覧: 継続＋当月単発の2件
	status, body = doJSON(t, http.MethodGet, "/direct-transfers?month="+month, taro, nil)
	if status != http.StatusOK {
		t.Fatalf("direct list status = %d, body = %s", status, body)
	}
	var list struct {
		DirectTransfers []struct {
			ID string `json:"id"`
		} `json:"directTransfers"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.DirectTransfers) != 2 {
		t.Fatalf("direct一覧 = %d件, want 2 (body = %s)", len(list.DirectTransfers), body)
	}

	// 給与同額・支出なし → 精算分0。立替精算純額 太郎→花子 (5000-2000)=3000。
	setSalaries(t, taro, taroID, hanako, hanakoID, month, 100000, 100000)

	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	type transfer struct {
		From      string `json:"from"`
		To        string `json:"to"`
		AmountYen int64  `json:"amountYen"`
	}
	var s struct {
		Transfer               *transfer `json:"transfer"`
		SettlementTransfer     *transfer `json:"settlementTransfer"`
		DirectTransfer         *transfer `json:"directTransfer"`
		TotalDirectTransferYen int64     `json:"totalDirectTransferYen"`
	}
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.SettlementTransfer != nil {
		t.Errorf("settlementTransfer = %+v, want null", s.SettlementTransfer)
	}
	if s.Transfer == nil || s.Transfer.From != taroID || s.Transfer.To != hanakoID || s.Transfer.AmountYen != 3000 {
		t.Errorf("transfer = %+v, want taro→hanako 3000", s.Transfer)
	}
	if s.DirectTransfer == nil || s.DirectTransfer.AmountYen != 3000 {
		t.Errorf("directTransfer = %+v, want taro→hanako 3000", s.DirectTransfer)
	}
	if s.TotalDirectTransferYen != 7000 {
		t.Errorf("totalDirectTransferYen = %d, want 7000", s.TotalDirectTransferYen)
	}

	// 更新（金額・内容。継続/単発の別と対象月は維持される）
	status, body = doJSON(t, http.MethodPut, "/direct-transfers/"+recurring.ID, taro, map[string]any{
		"from": taroID, "amountYen": 6000, "description": "お小遣い(増額)",
	})
	if status != http.StatusOK {
		t.Fatalf("direct update status = %d, body = %s", status, body)
	}
	var updated struct {
		AmountYen   int64  `json:"amountYen"`
		Description string `json:"description"`
		Recurring   bool   `json:"recurring"`
		To          string `json:"to"`
	}
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated.AmountYen != 6000 || updated.Description != "お小遣い(増額)" || !updated.Recurring || updated.To != hanakoID {
		t.Errorf("更新結果 = %+v, want 6000/お小遣い(増額)/recurring/to=hanako", updated)
	}

	// 更新が精算へ反映される: 純額 太郎→花子 (6000-2000)=4000
	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement(更新後) status = %d, body = %s", status, body)
	}
	s.Transfer, s.DirectTransfer = nil, nil
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.DirectTransfer == nil || s.DirectTransfer.From != taroID || s.DirectTransfer.AmountYen != 4000 {
		t.Errorf("更新後 directTransfer = %+v, want taro→hanako 4000", s.DirectTransfer)
	}

	// 後片付け: 登録した立替精算を削除する
	if status, _ := doJSON(t, http.MethodDelete, "/direct-transfers/"+recurring.ID, taro, nil); status != http.StatusNoContent {
		t.Errorf("delete(継続) status = %d, want 204", status)
	}
	if status, _ := doJSON(t, http.MethodDelete, "/direct-transfers/"+oneOff.ID, taro, nil); status != http.StatusNoContent {
		t.Errorf("delete(単発) status = %d, want 204", status)
	}
	if status, _ := doJSON(t, http.MethodDelete, "/direct-transfers/"+recurring.ID, taro, nil); status != http.StatusNotFound {
		t.Errorf("再削除 status = %d, want 404", status)
	}
}
