package web_test

// 精算設定（比重）API のテスト。

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestWeightAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	token := login(t, srv, acc[0].LoginID, acc[0].Password)

	resp, body := doJSON(t, http.MethodGet, srv.URL+"/settings/weight", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var weights struct {
		Weights map[string]int64 `json:"weights"`
	}
	if err := json.Unmarshal(body, &weights); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if weights.Weights[acc[0].AccountID] != 1 || weights.Weights[acc[1].AccountID] != 1 {
		t.Errorf("weights = %v, want 1:1", weights.Weights)
	}

	resp, body = doJSON(t, http.MethodPut, srv.URL+"/settings/weight", token, map[string]any{
		"weights": map[string]int64{acc[0].AccountID: 3, acc[1].AccountID: 2},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, &weights); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if weights.Weights[acc[0].AccountID] != 3 || weights.Weights[acc[1].AccountID] != 2 {
		t.Errorf("weights = %v, want 3:2", weights.Weights)
	}
}
