//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestWeightSettings は精算比重の更新が永続化されることを検証する。
// 比重はグローバル設定のため、終了時に既定(1:1)へ戻す。
func TestWeightSettings(t *testing.T) {
	waitForHealthy(t)
	taro, taroID := login(t, "taro", "taro-password")
	_, hanakoID := login(t, "hanako", "hanako-password")

	defer func() {
		if status, _ := doJSON(t, http.MethodPut, "/settings/weight", taro, map[string]any{
			"weights": map[string]int64{taroID: 1, hanakoID: 1},
		}); status != http.StatusOK {
			t.Errorf("weight reset status = %d, want 200", status)
		}
	}()

	status, body := doJSON(t, http.MethodPut, "/settings/weight", taro, map[string]any{
		"weights": map[string]int64{taroID: 2, hanakoID: 1},
	})
	if status != http.StatusOK {
		t.Fatalf("weight put status = %d, body = %s", status, body)
	}

	status, body = doJSON(t, http.MethodGet, "/settings/weight", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("weight get status = %d, body = %s", status, body)
	}
	var weights struct {
		Weights map[string]int64 `json:"weights"`
	}
	if err := json.Unmarshal(body, &weights); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if weights.Weights[taroID] != 2 || weights.Weights[hanakoID] != 1 {
		t.Errorf("weights = %v, want taro:2 hanako:1", weights.Weights)
	}
}

// TestClosingDaySetting は締め日の取得・更新の往復を検証する（設定値のみ。集計への影響は settlement_test.go）。
// グローバル設定のため終了時に既定(1)へ戻す。
func TestClosingDaySetting(t *testing.T) {
	waitForHealthy(t)
	taro, _ := login(t, "taro", "taro-password")

	defer func() {
		if status, _ := doJSON(t, http.MethodPut, "/settings/closing-day", taro, map[string]any{"closingDay": 1}); status != http.StatusOK {
			t.Errorf("closing-day reset status = %d, want 200", status)
		}
	}()

	// 取得（1〜31 の範囲）
	status, body := doJSON(t, http.MethodGet, "/settings/closing-day", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("closing-day get status = %d, body = %s", status, body)
	}
	var cd struct {
		ClosingDay int `json:"closingDay"`
	}
	if err := json.Unmarshal(body, &cd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cd.ClosingDay < 1 || cd.ClosingDay > 31 {
		t.Errorf("closingDay = %d, want 1〜31", cd.ClosingDay)
	}

	// 更新の往復
	status, body = doJSON(t, http.MethodPut, "/settings/closing-day", taro, map[string]any{"closingDay": 20})
	if status != http.StatusOK {
		t.Fatalf("closing-day put status = %d, body = %s", status, body)
	}
	if err := json.Unmarshal(body, &cd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cd.ClosingDay != 20 {
		t.Errorf("更新後 closingDay = %d, want 20", cd.ClosingDay)
	}
}
