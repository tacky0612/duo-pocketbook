//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

type reservationResp struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	Recurring          bool     `json:"recurring"`
	Status             string   `json:"status"`
	FulfilledIDs       []string `json:"fulfilledIds"`
	FulfilledAmountYen int64    `json:"fulfilledAmountYen"`
}

// listReservations は指定月の予約一覧を ID → 予約 のマップで返す。
// DynamoDB Local は実行間でデータを共有しうるため、件数ではなく作成したIDで照合する。
func listReservations(t *testing.T, token, month string) map[string]reservationResp {
	t.Helper()
	status, body := doJSON(t, http.MethodGet, "/reservations?month="+month, token, nil)
	if status != http.StatusOK {
		t.Fatalf("reservation list status = %d, body = %s", status, body)
	}
	var list struct {
		Reservations []reservationResp `json:"reservations"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := map[string]reservationResp{}
	for _, r := range list.Reservations {
		out[r.ID] = r
	}
	return out
}

// TestReservation は予約の登録・一覧・金額入力（共有支出/収入として登録）・スキップ・削除と、
// 入力した金額が精算に反映されることを DynamoDB Local 上で検証する。
func TestReservation(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	const month = "2044-02"
	setSalaries(t, taro, taroID, hanako, hanakoID, month, 100000, 100000)

	register := func(body map[string]any) reservationResp {
		t.Helper()
		status, resp := doJSON(t, http.MethodPost, "/reservations", taro, body)
		if status != http.StatusCreated {
			t.Fatalf("reservation post status = %d, body = %s", status, resp)
		}
		var r reservationResp
		if err := json.Unmarshal(resp, &r); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		return r
	}
	power := register(map[string]any{"kind": "expense", "memberId": taroID, "description": "電気代", "estimatedAmountYen": 8000, "month": "", "startMonth": month})
	bonus := register(map[string]any{"kind": "income", "memberId": hanakoID, "description": "賞与", "month": month})
	t.Cleanup(func() {
		doJSON(t, http.MethodDelete, "/reservations/"+power.ID, taro, nil)
		doJSON(t, http.MethodDelete, "/reservations/"+bonus.ID, taro, nil)
	})
	if !power.Recurring || bonus.Recurring {
		t.Fatalf("recurring: power=%v bonus=%v", power.Recurring, bonus.Recurring)
	}

	got := listReservations(t, taro, month)
	if got[power.ID].Status != "pending" || got[bonus.ID].Status != "pending" {
		t.Fatalf("登録直後は未入力のはず: %+v", got)
	}
	// 毎月の予約は開始月より前の月に現れない。
	if _, ok := listReservations(t, taro, "2044-01")[power.ID]; ok {
		t.Error("毎月の予約が開始月より前の月に現れた")
	}
	// 単発の予約は対象月以外に現れない。
	if _, ok := listReservations(t, taro, "2044-03")[bonus.ID]; ok {
		t.Error("単発の予約が別の月に現れた")
	}

	// 電気代を入力 → 共有支出として登録され精算に反映される（太郎が 6,000 立替 → 花子が 3,000 振込）。
	status, body := doJSON(t, http.MethodPost, "/reservations/"+power.ID+"/fulfill", taro, map[string]any{
		"month": month, "amountYen": 6000, "date": month + "-20",
	})
	if status != http.StatusCreated {
		t.Fatalf("fulfill status = %d, body = %s", status, body)
	}
	var fulfilled reservationResp
	if err := json.Unmarshal(body, &fulfilled); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fulfilled.Status != "fulfilled" || fulfilled.FulfilledAmountYen != 6000 {
		t.Fatalf("fulfilled = %+v", fulfilled)
	}
	t.Cleanup(func() { doJSON(t, http.MethodDelete, "/expenses/"+fulfilled.FulfilledIDs[0], taro, nil) })

	// 同じ月への二重入力は拒否され、支出は1件のまま。
	if status, body = doJSON(t, http.MethodPost, "/reservations/"+power.ID+"/fulfill", hanako, map[string]any{
		"month": month, "amountYen": 6000, "date": month + "-21",
	}); status != http.StatusBadRequest {
		t.Errorf("duplicate fulfill status = %d, body = %s", status, body)
	}

	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	var s struct {
		TotalExpenseYen int64 `json:"totalExpenseYen"`
		Transfer        *struct {
			From      string `json:"from"`
			AmountYen int64  `json:"amountYen"`
		} `json:"transfer"`
	}
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.TotalExpenseYen != 6000 || s.Transfer == nil || s.Transfer.From != hanakoID || s.Transfer.AmountYen != 3000 {
		t.Errorf("settlement = %s", body)
	}

	// 今月だけの賞与は「今月はなし」にできない（400）。毎月の電気代は翌月をスキップできる。
	if status, body = doJSON(t, http.MethodPut, "/reservations/"+bonus.ID+"/skip", taro, map[string]any{
		"month": month, "skipped": true,
	}); status != http.StatusBadRequest {
		t.Errorf("one-off skip status = %d, body = %s", status, body)
	}
	if status, body = doJSON(t, http.MethodPut, "/reservations/"+power.ID+"/skip", taro, map[string]any{
		"month": "2044-03", "skipped": true,
	}); status != http.StatusOK {
		t.Fatalf("skip status = %d, body = %s", status, body)
	}
	got = listReservations(t, taro, month)
	if got[power.ID].Status != "fulfilled" || got[bonus.ID].Status != "pending" {
		t.Errorf("statuses = %+v", got)
	}
	if st := listReservations(t, taro, "2044-03")[power.ID].Status; st != "skipped" {
		t.Errorf("2044-03 status = %q, want skipped", st)
	}

	// 予約から登録した支出を削除すると未入力に戻る。
	if status, body = doJSON(t, http.MethodDelete, "/expenses/"+fulfilled.FulfilledIDs[0], taro, nil); status != http.StatusNoContent {
		t.Fatalf("expense delete status = %d, body = %s", status, body)
	}
	if st := listReservations(t, taro, month)[power.ID].Status; st != "pending" {
		t.Errorf("支出削除後の status = %q, want pending", st)
	}

	// 頻度の変更: 毎月の電気代を当月のみへ → IDは変わらず、翌月には現れなくなる。
	status, body = doJSON(t, http.MethodPut, "/reservations/"+power.ID, taro, map[string]any{
		"memberId": taroID, "description": "電気代", "estimatedAmountYen": 8000, "month": month,
	})
	if status != http.StatusOK {
		t.Fatalf("update frequency status = %d, body = %s", status, body)
	}
	var moved reservationResp
	if err := json.Unmarshal(body, &moved); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if moved.Recurring || moved.ID != power.ID {
		t.Errorf("moved = %+v（同じIDの単発になるべき）", moved)
	}
	if _, ok := listReservations(t, taro, month)[moved.ID]; !ok {
		t.Error("頻度変更後の予約が当月に現れない")
	}
	if _, ok := listReservations(t, taro, "2044-03")[moved.ID]; ok {
		t.Error("当月のみにした予約が翌月に現れた")
	}

	// 削除すると一覧から消える。
	if status, body = doJSON(t, http.MethodDelete, "/reservations/"+bonus.ID, taro, nil); status != http.StatusNoContent {
		t.Fatalf("reservation delete status = %d, body = %s", status, body)
	}
	if _, ok := listReservations(t, taro, month)[bonus.ID]; ok {
		t.Error("削除した予約が一覧に残っている")
	}
}
