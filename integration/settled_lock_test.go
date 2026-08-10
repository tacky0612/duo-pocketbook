//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestSettledMonthEditLock は、精算確定済みの月に属するデータの作成/更新/削除が
// 409 MONTH_SETTLED で拒否されること、対象月に紐づかない項目（別月・毎月継続・固定費）は
// 引き続き編集できること、精算を取り消すと再び編集できるようになることを検証する。
func TestSettledMonthEditLock(t *testing.T) {
	waitForHealthy(t)
	taro, taroID, hanako, hanakoID := loginBoth(t)

	// 他テストと衝突しない専用の月を使う。
	const month = "2041-09"
	const otherMonth = "2041-10"

	// POST してレスポンスの id を返す（201 を期待）。
	postID := func(name, path string, body map[string]any) string {
		t.Helper()
		status, resp := doJSON(t, http.MethodPost, path, taro, body)
		if status != http.StatusCreated {
			t.Fatalf("%s: status = %d, want 201 (body=%s)", name, status, resp)
		}
		var out struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(resp, &out); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		return out.ID
	}

	// 409 かつ error.code == MONTH_SETTLED を確認する。
	wantLocked := func(name string, status int, body []byte) {
		t.Helper()
		if status != http.StatusConflict {
			t.Errorf("%s: status = %d, want 409 (body=%s)", name, status, body)
			return
		}
		var e struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &e); err != nil {
			t.Errorf("%s: unmarshal: %v", name, err)
			return
		}
		if e.Error.Code != "MONTH_SETTLED" {
			t.Errorf("%s: code = %q, want MONTH_SETTLED (body=%s)", name, e.Error.Code, body)
		}
	}

	// --- 確定前に、更新/削除の対象となる当月データを用意する ---
	expID := postID("先行の支出", "/expenses", map[string]any{
		"paidBy": taroID, "amountYen": 3000, "description": "食費", "date": month + "-10",
	})
	incID := postID("先行の単発収入", "/incomes", map[string]any{
		"memberId": taroID, "amountYen": 5000, "description": "副業", "month": month,
	})
	dtID := postID("先行の単発立替", "/direct-transfers", map[string]any{
		"from": taroID, "amountYen": 2000, "description": "立替", "month": month,
	})

	// 給与を入れて当月を精算確定する。
	setSalaries(t, taro, taroID, hanako, hanakoID, month, 100000, 100000)
	if status, body := doJSON(t, http.MethodPut, "/months/"+month+"/settlement/status", taro, map[string]any{"settled": true}); status != http.StatusOK {
		t.Fatalf("settle status = %d, body = %s", status, body)
	}

	// --- 確定済み月への操作はすべて 409 MONTH_SETTLED ---
	status, body := doJSON(t, http.MethodPost, "/expenses", taro, map[string]any{
		"paidBy": taroID, "amountYen": 1000, "description": "追加", "date": month + "-20",
	})
	wantLocked("支出POST(確定月)", status, body)

	status, body = doJSON(t, http.MethodPut, "/expenses/"+expID, taro, map[string]any{
		"paidBy": taroID, "amountYen": 9999, "description": "変更", "date": month + "-10",
	})
	wantLocked("支出PUT(確定月)", status, body)

	status, body = doJSON(t, http.MethodDelete, "/expenses/"+expID, taro, nil)
	wantLocked("支出DELETE(確定月)", status, body)

	status, body = doJSON(t, http.MethodPut, "/months/"+month+"/salaries/"+taroID, taro, map[string]any{"amountYen": 120000})
	wantLocked("給与PUT(確定月)", status, body)

	status, body = doJSON(t, http.MethodPost, "/incomes", taro, map[string]any{
		"memberId": taroID, "amountYen": 3000, "description": "副業2", "month": month,
	})
	wantLocked("収入POST(確定月・単発)", status, body)

	status, body = doJSON(t, http.MethodPut, "/incomes/"+incID, taro, map[string]any{
		"memberId": taroID, "amountYen": 8000, "description": "変更",
	})
	wantLocked("収入PUT(確定月・単発)", status, body)

	status, body = doJSON(t, http.MethodDelete, "/incomes/"+incID, taro, nil)
	wantLocked("収入DELETE(確定月・単発)", status, body)

	status, body = doJSON(t, http.MethodPost, "/direct-transfers", taro, map[string]any{
		"from": taroID, "amountYen": 500, "description": "立替2", "month": month,
	})
	wantLocked("立替POST(確定月・単発)", status, body)

	status, body = doJSON(t, http.MethodPut, "/direct-transfers/"+dtID, taro, map[string]any{
		"from": taroID, "amountYen": 700, "description": "変更",
	})
	wantLocked("立替PUT(確定月・単発)", status, body)

	status, body = doJSON(t, http.MethodDelete, "/direct-transfers/"+dtID, taro, nil)
	wantLocked("立替DELETE(確定月・単発)", status, body)

	// --- 対象月に紐づかない項目・別の未確定月は引き続き編集できる ---
	// 別の未確定月の支出は登録でき、削除もできる。
	otherExpID := postID("別月の支出", "/expenses", map[string]any{
		"paidBy": taroID, "amountYen": 1000, "description": "翌月", "date": otherMonth + "-05",
	})
	if status, body := doJSON(t, http.MethodDelete, "/expenses/"+otherExpID, taro, nil); status != http.StatusNoContent {
		t.Errorf("別月の支出DELETE status = %d, want 204 (body=%s)", status, body)
	}

	// 毎月継続の収入・立替、固定費はグローバル資源のため、登録できることを確認して即削除する。
	recIncID := postID("継続収入", "/incomes", map[string]any{
		"memberId": taroID, "amountYen": 400, "description": "継続収入", "month": "",
	})
	if status, body := doJSON(t, http.MethodDelete, "/incomes/"+recIncID, taro, nil); status != http.StatusNoContent {
		t.Errorf("継続収入DELETE status = %d, want 204 (body=%s)", status, body)
	}

	recDtID := postID("継続立替", "/direct-transfers", map[string]any{
		"from": taroID, "amountYen": 300, "description": "継続立替", "month": "",
	})
	if status, body := doJSON(t, http.MethodDelete, "/direct-transfers/"+recDtID, taro, nil); status != http.StatusNoContent {
		t.Errorf("継続立替DELETE status = %d, want 204 (body=%s)", status, body)
	}

	fixedID := postID("固定費", "/recurring-expenses", map[string]any{
		"paidBy": taroID, "amountYen": 40000, "description": "家賃",
	})
	if status, body := doJSON(t, http.MethodDelete, "/recurring-expenses/"+fixedID, taro, nil); status != http.StatusNoContent {
		t.Errorf("固定費DELETE status = %d, want 204 (body=%s)", status, body)
	}

	// --- 精算を取り消すと再び編集できる（併せて先行データを後始末する） ---
	if status, body := doJSON(t, http.MethodPut, "/months/"+month+"/settlement/status", taro, map[string]any{"settled": false}); status != http.StatusOK {
		t.Fatalf("unsettle status = %d, body = %s", status, body)
	}
	if status, body := doJSON(t, http.MethodDelete, "/expenses/"+expID, taro, nil); status != http.StatusNoContent {
		t.Errorf("取り消し後の支出DELETE status = %d, want 204 (body=%s)", status, body)
	}
	if status, body := doJSON(t, http.MethodDelete, "/incomes/"+incID, taro, nil); status != http.StatusNoContent {
		t.Errorf("取り消し後の収入DELETE status = %d, want 204 (body=%s)", status, body)
	}
	if status, body := doJSON(t, http.MethodDelete, "/direct-transfers/"+dtID, taro, nil); status != http.StatusNoContent {
		t.Errorf("取り消し後の立替DELETE status = %d, want 204 (body=%s)", status, body)
	}
}
