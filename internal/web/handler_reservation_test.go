package web_test

// 予約 API のテスト。

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestReservationAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	taro, hanako := acc[0], acc[1]
	token := login(t, srv, taro.LoginID, taro.Password)

	type reservation struct {
		ID                 string   `json:"id"`
		Kind               string   `json:"kind"`
		Recurring          bool     `json:"recurring"`
		Month              string   `json:"month"`
		Status             string   `json:"status"`
		FulfilledIDs       []string `json:"fulfilledIds"`
		FulfilledAmountYen int64    `json:"fulfilledAmountYen"`
		EstimatedAmountYen int64    `json:"estimatedAmountYen"`
	}
	decode := func(body []byte, v any) {
		t.Helper()
		if err := json.Unmarshal(body, v); err != nil {
			t.Fatalf("unmarshal: %v (%s)", err, body)
		}
	}

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/reservations", token, map[string]any{
		"kind": "expense", "memberId": taro.AccountID, "description": "電気代", "estimatedAmountYen": 8000, "month": "",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var power reservation
	decode(body, &power)
	if !power.Recurring || power.Status != "pending" || power.EstimatedAmountYen != 8000 {
		t.Errorf("created = %+v", power)
	}

	resp, body = doJSON(t, http.MethodPost, srv.URL+"/reservations", token, map[string]any{
		"kind": "income", "memberId": hanako.AccountID, "description": "賞与", "month": "2026-07",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var bonus reservation
	decode(body, &bonus)

	// 不正な種別は 400。
	if resp, body = doJSON(t, http.MethodPost, srv.URL+"/reservations", token, map[string]any{
		"kind": "x", "memberId": taro.AccountID, "description": "x",
	}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid kind: status = %d, body = %s", resp.StatusCode, body)
	}

	var list struct {
		Month        string        `json:"month"`
		Reservations []reservation `json:"reservations"`
		PendingCount int           `json:"pendingCount"`
	}
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/reservations?month=2026-07", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	decode(body, &list)
	if len(list.Reservations) != 2 || list.PendingCount != 2 {
		t.Fatalf("list = %+v", list)
	}

	// 支出の予約を入力 → 共有支出に reservationId 付きで現れる。
	resp, body = doJSON(t, http.MethodPost, srv.URL+"/reservations/"+power.ID+"/fulfill", token, map[string]any{
		"month": "2026-07", "amountYen": 7820, "date": "2026-07-25",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("fulfill status = %d, body = %s", resp.StatusCode, body)
	}
	var fulfilled reservation
	decode(body, &fulfilled)
	if fulfilled.Status != "fulfilled" || fulfilled.FulfilledAmountYen != 7820 || len(fulfilled.FulfilledIDs) != 1 {
		t.Errorf("fulfilled = %+v", fulfilled)
	}
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/expenses?month=2026-07", token, nil)
	var expenses struct {
		Expenses []struct {
			ID            string `json:"id"`
			ReservationID string `json:"reservationId"`
		} `json:"expenses"`
	}
	decode(body, &expenses)
	if len(expenses.Expenses) != 1 || expenses.Expenses[0].ReservationID != power.ID || expenses.Expenses[0].ID != fulfilled.FulfilledIDs[0] {
		t.Errorf("expenses = %+v", expenses)
	}

	// 今月だけの予約は「今月はなし」にできない（400）。毎月の予約はできる。
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/reservations/"+bonus.ID+"/skip", token, map[string]any{
		"month": "2026-07", "skipped": true,
	}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("one-off skip status = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, http.MethodPut, srv.URL+"/reservations/"+power.ID+"/skip", token, map[string]any{
		"month": "2026-08", "skipped": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("skip status = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/reservations?month=2026-08&kind=expense", token, nil)
	list.Reservations = nil
	decode(body, &list)
	if len(list.Reservations) != 1 || list.Reservations[0].Status != "skipped" || list.PendingCount != 0 {
		t.Errorf("expense list (2026-08) = %+v", list)
	}
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/reservations?month=2026-07&kind=income", token, nil)
	list.Reservations = nil
	decode(body, &list)
	if len(list.Reservations) != 1 || list.Reservations[0].Status != "pending" || list.PendingCount != 1 {
		t.Errorf("income list = %+v", list)
	}

	// 更新・削除。
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/reservations/"+power.ID, token, map[string]any{
		"memberId": hanako.AccountID, "description": "電気代", "estimatedAmountYen": 9000,
	}); resp.StatusCode != http.StatusOK {
		t.Errorf("update status = %d, body = %s", resp.StatusCode, body)
	}
	if resp, body = doJSON(t, http.MethodDelete, srv.URL+"/reservations/"+bonus.ID, token, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, body = %s", resp.StatusCode, body)
	}
	if resp, body = doJSON(t, http.MethodDelete, srv.URL+"/reservations/"+bonus.ID, token, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("delete again status = %d, body = %s", resp.StatusCode, body)
	}
}
