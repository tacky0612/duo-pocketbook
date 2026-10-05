package web_test

// ルーター（CORS プリフライト）のテスト。

import (
	"net/http"
	"testing"
)

func TestCORSPreflight(t *testing.T) {
	srv, _ := newTestServer(t)

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/expenses", nil)
	req.Header.Set("Origin", "https://example.github.io")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://example.github.io" {
		t.Errorf("Allow-Origin = %q", got)
	}
}
