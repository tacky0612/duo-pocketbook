//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// getUnsettled は GET /settlements/unsettled?before=<before> を呼び、返った月を返す（null なら空文字）。
func getUnsettled(t *testing.T, token, before string) string {
	t.Helper()
	status, body := doJSON(t, http.MethodGet, "/settlements/unsettled?before="+before, token, nil)
	if status != http.StatusOK {
		t.Fatalf("unsettled(before=%s) status = %d, body = %s", before, status, body)
	}
	var res struct {
		Month *string `json:"month"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("unmarshal: %v (body = %s)", err, body)
	}
	if res.Month == nil {
		return ""
	}
	return *res.Month
}

// createID は POST で作成し、レスポンスの id を返す。
func createID(t *testing.T, path, token string, body map[string]any) string {
	t.Helper()
	status, resBody := doJSON(t, http.MethodPost, path, token, body)
	if status != http.StatusCreated {
		t.Fatalf("POST %s status = %d, body = %s", path, status, resBody)
	}
	var res struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resBody, &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return res.ID
}

// TestOldestUnsettledMonth は未精算月の取得（初期表示月の決定に使う）の仕様を検証する。
//
//   - before より前で、データがあるのに未精算の「最も古い」月を返す（before 自身は対象外）
//   - データは給与・共有支出・単発の追加収入・単発の立替精算のいずれでもよい
//   - 固定費・継続の追加収入・継続の立替精算だけではデータありとみなさない
//   - 精算済みの月に到達したらそれより前は遡らない
//   - 最大12か月まで遡る
//
// 他テストの月と衝突しないよう 2059〜2061 年を使う（前後12か月にも他テストのデータは無い）。
func TestOldestUnsettledMonth(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	expect := func(before, want string) {
		t.Helper()
		if got := getUnsettled(t, taro, before); got != want {
			t.Errorf("before=%s: month = %q, want %q", before, got, want)
		}
	}

	// --- データが無ければ null ---
	expect("2061-01", "")

	// --- 全月に効くデータ（固定費・継続の追加収入・継続の立替精算）だけでは対象にしない ---
	// いずれもグローバル資源のため、テスト終了時に必ず削除する。
	recurringExpense := createID(t, "/recurring-expenses", taro, map[string]any{
		"paidBy": taroID, "amountYen": 50000, "description": "未精算月テスト・固定費",
	})
	recurringIncome := createID(t, "/incomes", taro, map[string]any{
		"memberId": taroID, "amountYen": 10000, "description": "未精算月テスト・継続収入", "month": "",
	})
	recurringTransfer := createID(t, "/direct-transfers", taro, map[string]any{
		"from": taroID, "amountYen": 1000, "description": "未精算月テスト・継続立替",
	})
	defer func() {
		for _, path := range []string{
			"/recurring-expenses/" + recurringExpense,
			"/incomes/" + recurringIncome,
			"/direct-transfers/" + recurringTransfer,
		} {
			if status, body := doJSON(t, http.MethodDelete, path, taro, nil); status != http.StatusNoContent {
				t.Errorf("cleanup DELETE %s status = %d, body = %s", path, status, body)
			}
		}
	}()
	expect("2061-01", "")

	// --- 月に紐づくデータの種類ごとに検出され、常に最も古い月が返る ---
	// 新しい月から順に追加し、そのたびに追加した（より古い）月が返ることを確認する。
	// 給与（片方のみでもデータあり）
	if status, body := doJSON(t, http.MethodPut, "/months/2060-09/salaries/"+taroID, taro, map[string]any{"amountYen": 100000}); status != http.StatusOK {
		t.Fatalf("salary status = %d, body = %s", status, body)
	}
	expect("2061-01", "2060-09")
	// 単発の立替精算
	createID(t, "/direct-transfers", taro, map[string]any{
		"from": taroID, "amountYen": 2000, "description": "単発立替", "month": "2060-07",
	})
	expect("2061-01", "2060-07")
	// 単発の追加収入
	createID(t, "/incomes", hanako, map[string]any{
		"memberId": hanakoID, "amountYen": 30000, "description": "単発収入", "month": "2060-05",
	})
	expect("2061-01", "2060-05")
	// 共有支出
	createID(t, "/expenses", hanako, map[string]any{
		"paidBy": hanakoID, "amountYen": 3000, "description": "食費", "date": "2060-03-10",
	})
	expect("2061-01", "2060-03")

	// --- before 自身は対象外（それより前にデータが無ければ null） ---
	expect("2060-03", "")
	expect("2060-04", "2060-03")

	// --- 最大12か月まで遡る ---
	// before=2061-03 は 2061-02〜2060-03 の12か月 → 2060-03 が範囲内
	expect("2061-03", "2060-03")
	// before=2061-04 は 2061-03〜2060-04 → 2060-03 は範囲外、範囲内で最も古いのは 2060-05
	expect("2061-04", "2060-05")
	// before=2061-10 は 2061-09〜2060-10 → データなし
	expect("2061-10", "")

	// --- 精算済みの月に到達したら遡りを打ち切る ---
	// 2060-06 を精算済みにすると、それより前の 2060-05・2060-03 は返らない
	setSalaries(t, taro, taroID, hanako, hanakoID, "2060-06", 100000, 100000)
	if status, body := doJSON(t, http.MethodPut, "/months/2060-06/settlement/status", taro, map[string]any{"settled": true}); status != http.StatusOK {
		t.Fatalf("settle status = %d, body = %s", status, body)
	}
	expect("2061-01", "2060-07")
	// 精算済みの月そのものが基準月の直前なら null（精算済み月の手前で即終了）
	expect("2060-07", "")

	// 精算済みの月の後ろの未精算月も精算すると、残りは 2060-09 だけになる
	setSalaries(t, taro, taroID, hanako, hanakoID, "2060-07", 100000, 100000)
	if status, body := doJSON(t, http.MethodPut, "/months/2060-07/settlement/status", taro, map[string]any{"settled": true}); status != http.StatusOK {
		t.Fatalf("settle status = %d, body = %s", status, body)
	}
	expect("2061-01", "2060-09")

	// 精算を取り消すと、再び対象に戻る（2060-06 で打ち切られるため 2060-07 が最古）
	if status, body := doJSON(t, http.MethodPut, "/months/2060-07/settlement/status", taro, map[string]any{"settled": false}); status != http.StatusOK {
		t.Fatalf("unsettle status = %d, body = %s", status, body)
	}
	expect("2061-01", "2060-07")
}

// TestOldestUnsettledMonthClosingDay は、共有支出のデータ判定が締め日に基づく精算月で行われることを検証する。
// 締め日はグローバル設定のため、終了時に既定(1)へ戻す。2062〜2063 年を使う。
func TestOldestUnsettledMonthClosingDay(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, _, _ := loginBoth(t)

	// 2063-10-20 の支出: 締め日=1 なら 2063-10 分、締め日=15 なら 2063-11 分
	createID(t, "/expenses", taro, map[string]any{
		"paidBy": taroID, "amountYen": 5000, "description": "締め日テスト", "date": "2063-10-20",
	})

	if got := getUnsettled(t, taro, "2063-12"); got != "2063-10" {
		t.Errorf("締め日=1: month = %q, want 2063-10", got)
	}

	defer func() {
		if status, body := doJSON(t, http.MethodPut, "/settings/closing-day", taro, map[string]any{"closingDay": 1}); status != http.StatusOK {
			t.Errorf("closing-day reset status = %d, body = %s", status, body)
		}
	}()
	if status, body := doJSON(t, http.MethodPut, "/settings/closing-day", taro, map[string]any{"closingDay": 15}); status != http.StatusOK {
		t.Fatalf("closing-day put status = %d, body = %s", status, body)
	}
	if got := getUnsettled(t, taro, "2063-12"); got != "2063-11" {
		t.Errorf("締め日=15: month = %q, want 2063-11", got)
	}
	// 精算月 2063-11 を基準にすると、それより前にデータは無い
	if got := getUnsettled(t, taro, "2063-11"); got != "" {
		t.Errorf("締め日=15・before=2063-11: month = %q, want null", got)
	}
}

// TestOldestUnsettledMonthValidation は未精算月取得の入力検証と認証を検証する。
func TestOldestUnsettledMonthValidation(t *testing.T) {
	waitForHealthy(t)
	taro, _ := auth(t, "taro", "taro-password")

	for _, q := range []string{"", "?before=", "?before=bad", "?before=2060-13"} {
		if status, body := doJSON(t, http.MethodGet, "/settlements/unsettled"+q, taro, nil); status != http.StatusBadRequest {
			t.Errorf("query %q: status = %d, want 400 (body = %s)", q, status, body)
		}
	}
	if status, _ := doJSON(t, http.MethodGet, "/settlements/unsettled?before=2060-01", "", nil); status != http.StatusUnauthorized {
		t.Errorf("トークンなし: status = %d, want 401", status)
	}
}
