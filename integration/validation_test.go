//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"
)

// TestValidationErrors は各エンドポイントの入力検証（400/404）を横断的に確認する。
// いずれも状態を変更しない失敗ケースのため、月・グローバル設定への副作用はない。
func TestValidationErrors(t *testing.T) {
	waitForHealthy(t)
	taro, taroID := auth(t, "taro", "taro-password")

	cases := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{"支出: 負の金額", http.MethodPost, "/expenses", map[string]any{"paidBy": taroID, "amountYen": -1, "date": testMonth + "-01"}, 400},
		{"支出: 不明メンバー", http.MethodPost, "/expenses", map[string]any{"paidBy": "nobody", "amountYen": 100, "date": testMonth + "-01"}, 400},
		{"支出: 月形式不正", http.MethodGet, "/expenses?month=bad", nil, 400},
		{"給与: 月形式不正", http.MethodPut, "/months/bad/salaries/" + taroID, map[string]any{"amountYen": 1}, 400},
		{"追加収入: 内容なし", http.MethodPost, "/incomes", map[string]any{"memberId": taroID, "amountYen": 100}, 400},
		{"追加収入: 不明メンバー", http.MethodPost, "/incomes", map[string]any{"memberId": "nobody", "amountYen": 100, "description": "副業"}, 400},
		{"立替精算: 金額0", http.MethodPost, "/direct-transfers", map[string]any{"from": taroID, "amountYen": 0, "description": "x"}, 400},
		{"固定費: 内容なし", http.MethodPost, "/recurring-expenses", map[string]any{"paidBy": taroID, "amountYen": 100}, 400},
		{"締め日: 範囲外", http.MethodPut, "/settings/closing-day", map[string]any{"closingDay": 32}, 400},
		{"比重: 0以下", http.MethodPut, "/settings/weight", map[string]any{"weights": map[string]int64{taroID: 0}}, 400},
		{"存在しない支出の削除", http.MethodDelete, fmt.Sprintf("/expenses/%s_missing", testMonth), nil, 404},
		{"存在しない追加収入の削除", http.MethodDelete, "/incomes/inc_missing", nil, 404},
		{"存在しない立替精算の削除", http.MethodDelete, "/direct-transfers/dtr_missing", nil, 404},
		{"存在しない固定費の削除", http.MethodDelete, "/recurring-expenses/missing", nil, 404},
	}
	for _, tt := range cases {
		if status, body := doJSON(t, tt.method, tt.path, taro, tt.body); status != tt.wantStatus {
			t.Errorf("%s: status = %d, want %d (body = %s)", tt.name, status, tt.wantStatus, body)
		}
	}
}
