//go:build integration

// Package integration は Docker Compose で起動したローカル環境
// (アプリ + DynamoDB Local) に対するE2Eテスト。外部への通信は行わない。
//
// テストは機能ごとにファイルを分割している:
//
//	integration_test.go     共通ヘルパー（HTTP・ログイン・月）
//	auth_test.go            認証・メンバー・アカウント・プロフィール
//	expense_test.go         共有支出 CRUD
//	recurring_expense_test.go 固定費 CRUD と精算反映
//	salary_test.go          給与入力・一覧・精算の可否・基本精算
//	income_test.go          追加収入 CRUD と精算反映
//	direct_transfer_test.go 立替精算 CRUD と精算反映
//	settlement_test.go      締め日・履歴・精算済みフラグ
//	settings_test.go        精算比重・締め日設定
//	validation_test.go      バリデーションエラー
//
// 実行方法:
//
//	docker compose up -d --build --wait
//	go test -tags=integration ./integration/...
//
// 前提: 各実行は初期化された DynamoDB Local（インメモリ）に対して1回だけ走る
// （CI・make up ともに毎回 up し直す）。月スコープのデータはテストごとに別の月を
// 使って干渉を避け、全月に影響するグローバル資源（固定費・比重・締め日・
// 継続の立替精算/収入）は各テスト内で必ず後始末する。
package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

var baseURL = func() string {
	if v := os.Getenv("BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}()

// testMonth はテスト間の干渉を避けるため、実行ごとにユニークな月を返す。
// バリデーションなど「月スコープのデータを残さない」テストで使う。
var testMonth = func() string {
	base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	offset := time.Now().UnixNano() % 120
	return base.AddDate(0, int(offset), 0).Format("2006-01")
}()

func doJSON(t *testing.T, method, path, token string, body any) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, baseURL+path, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, data
}

func waitForHealthy(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(time.Second)
	}
	t.Fatalf("アプリが起動しませんでした: %s", baseURL)
}

// login はログインIDで認証し、トークンと不変の AccountID（member.id）を返す。
// 支出の paidBy・給与・比重などのキーはログインIDではなく AccountID を使う。
func login(t *testing.T, loginID, password string) (token, accountID string) {
	t.Helper()
	status, body := doJSON(t, http.MethodPost, "/login", "", map[string]string{
		"memberId": loginID, "password": password,
	})
	if status != http.StatusOK {
		t.Fatalf("login(%s) status = %d, body = %s", loginID, status, body)
	}
	var out struct {
		Token  string `json:"token"`
		Member struct {
			ID string `json:"id"`
		} `json:"member"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out.Token, out.Member.ID
}

// loginBoth は taro / hanako 双方でログインし、トークンとAccountIDを返す。
func loginBoth(t *testing.T) (taro, taroID, hanako, hanakoID string) {
	t.Helper()
	taro, taroID = login(t, "taro", "taro-password")
	hanako, hanakoID = login(t, "hanako", "hanako-password")
	return
}

// setSalaries は対象月の両メンバーの給与を入力する（精算可能な状態にする）。
func setSalaries(t *testing.T, taroToken, taroID, hanakoToken, hanakoID, month string, taroYen, hanakoYen int64) {
	t.Helper()
	if status, body := doJSON(t, http.MethodPut, "/months/"+month+"/salaries/"+taroID, taroToken, map[string]any{"amountYen": taroYen}); status != http.StatusOK {
		t.Fatalf("salary(taro) status = %d, body = %s", status, body)
	}
	if status, body := doJSON(t, http.MethodPut, "/months/"+month+"/salaries/"+hanakoID, hanakoToken, map[string]any{"amountYen": hanakoYen}); status != http.StatusOK {
		t.Fatalf("salary(hanako) status = %d, body = %s", status, body)
	}
}
