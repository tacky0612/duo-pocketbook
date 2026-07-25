//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestClosingDaySettlement は締め日設定に応じて支出の集計期間が暦月をまたぐことを検証する。
// 締め日はグローバル設定のため、終了時に既定(1)へ戻す。
func TestClosingDaySettlement(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	defer func() {
		if status, body := doJSON(t, http.MethodPut, "/settings/closing-day", taro, map[string]any{"closingDay": 1}); status != http.StatusOK {
			t.Fatalf("closing-day reset status = %d, body = %s", status, body)
		}
	}()

	// 締め日=15（6/15〜7/14 → 7月分）。他テストの月と衝突しない2035年で検証する。
	status, body := doJSON(t, http.MethodPut, "/settings/closing-day", taro, map[string]any{"closingDay": 15})
	if status != http.StatusOK {
		t.Fatalf("closing-day put status = %d, body = %s", status, body)
	}
	var cd struct {
		ClosingDay int `json:"closingDay"`
	}
	if err := json.Unmarshal(body, &cd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cd.ClosingDay != 15 {
		t.Fatalf("closingDay = %d, want 15", cd.ClosingDay)
	}

	reg := func(date string, yen int64) {
		if status, body := doJSON(t, http.MethodPost, "/expenses", taro, map[string]any{
			"paidBy": taroID, "amountYen": yen, "description": date, "date": date,
		}); status != http.StatusCreated {
			t.Fatalf("register(%s) status = %d, body = %s", date, status, body)
		}
	}
	reg("2035-06-14", 1000) // 6月分（除外）
	reg("2035-06-15", 2000) // 7月分（起算日）
	reg("2035-07-14", 4000) // 7月分（締め前日）
	reg("2035-07-15", 8000) // 8月分（除外）

	// 支出一覧も締め期間で集計される（7月 = 6/15・7/14 の2件）
	if list := listExpenses(t, taro, "2035-07"); len(list) != 2 {
		t.Fatalf("7月の支出件数 = %d, want 2 (%+v)", len(list), list)
	}
	// 8月には 7/15 分のみ
	if list := listExpenses(t, taro, "2035-08"); len(list) != 1 || list[0].Date != "2035-07-15" {
		t.Errorf("8月の支出 = %+v, want [2035-07-15]", list)
	}

	// 精算の合計支出は 2000+4000 = 6000
	setSalaries(t, taro, taroID, hanako, hanakoID, "2035-07", 100000, 100000)
	status, body = doJSON(t, http.MethodGet, "/months/2035-07/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	var s struct {
		TotalExpenseYen int64 `json:"totalExpenseYen"`
	}
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.TotalExpenseYen != 6000 {
		t.Errorf("7月の合計支出 = %d, want 6000", s.TotalExpenseYen)
	}
}

// TestSettlementHistory は精算完了した月のスナップショットが新しい月順に返り、
// 精算未完了の月が除外されることを検証する。
func TestSettlementHistory(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	settle := func(month string) {
		if status, body := doJSON(t, http.MethodPut, "/months/"+month+"/settlement/status", taro, map[string]any{"settled": true}); status != http.StatusOK {
			t.Fatalf("settle(%s) status = %d, body = %s", month, status, body)
		}
	}

	// 2037-01・2037-02 は精算完了、2037-03 は精算未完了（履歴から除外される）
	setSalaries(t, taro, taroID, hanako, hanakoID, "2037-01", 100000, 100000)
	setSalaries(t, taro, taroID, hanako, hanakoID, "2037-02", 100000, 100000)
	setSalaries(t, taro, taroID, hanako, hanakoID, "2037-03", 100000, 100000)
	settle("2037-01")
	settle("2037-02")

	status, body := doJSON(t, http.MethodGet, "/settlements/history?from=2037-01&to=2037-03", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("history status = %d, body = %s", status, body)
	}
	var res struct {
		Entries []struct {
			Month     string `json:"month"`
			SettledAt string `json:"settledAt"`
			Members   []struct {
				ID string `json:"id"`
			} `json:"members"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(res.Entries) != 2 {
		t.Fatalf("entries = %d, want 2（2037-03は精算未完了で除外）(body = %s)", len(res.Entries), body)
	}
	// 新しい月順
	if res.Entries[0].Month != "2037-02" || res.Entries[1].Month != "2037-01" {
		t.Errorf("順序 = [%s, %s], want [2037-02, 2037-01]", res.Entries[0].Month, res.Entries[1].Month)
	}
	// スナップショットには完了日時とメンバー内訳が含まれる
	if res.Entries[0].SettledAt == "" {
		t.Error("settledAt が空")
	}
	if len(res.Entries[0].Members) != 2 {
		t.Errorf("members = %d, want 2", len(res.Entries[0].Members))
	}

	// 不正な範囲（from > to）は 400
	if status, _ := doJSON(t, http.MethodGet, "/settlements/history?from=2037-03&to=2037-01", taro, nil); status != http.StatusBadRequest {
		t.Errorf("from>to status = %d, want 400", status)
	}
}

// TestSettlementStatus は精算済みフラグの更新と精算レスポンスへの反映を検証する。
func TestSettlementStatus(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	const month = "2038-08"
	setSalaries(t, taro, taroID, hanako, hanakoID, month, 100000, 100000)

	// 初期状態は未精算
	status, body := doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	var s struct {
		Settled bool `json:"settled"`
	}
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Settled {
		t.Errorf("初期 settled = true, want false")
	}

	// 精算済みに更新
	status, body = doJSON(t, http.MethodPut, "/months/"+month+"/settlement/status", taro, map[string]any{"settled": true})
	if status != http.StatusOK {
		t.Fatalf("status put status = %d, body = %s", status, body)
	}
	var st struct {
		Month   string `json:"month"`
		Settled bool   `json:"settled"`
	}
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if st.Month != month || !st.Settled {
		t.Errorf("status put レスポンス = %+v, want month=%s settled=true", st, month)
	}

	// 精算レスポンスに反映される
	status, body = doJSON(t, http.MethodGet, "/months/"+month+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement(更新後) status = %d, body = %s", status, body)
	}
	s.Settled = false
	if err := json.Unmarshal(body, &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !s.Settled {
		t.Errorf("更新後 settled = false, want true")
	}
}
