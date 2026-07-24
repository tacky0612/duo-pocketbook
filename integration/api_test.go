//go:build integration

// Package integration は Docker Compose で起動したローカル環境
// (アプリ + DynamoDB Local) に対するE2Eテスト。外部への通信は行わない。
//
// 実行方法:
//
//	docker compose up -d --build --wait
//	go test -tags=integration ./integration/...
package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// テスト間の干渉を避けるため、実行ごとにユニークな月を使う。
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
// 支出の paidBy・収入・比重などのキーはログインIDではなく AccountID を使う。
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

func TestE2EFlow(t *testing.T) {
	waitForHealthy(t)

	// --- 認証 ---
	taro, taroID := login(t, "taro", "taro-password")
	hanako, hanakoID := login(t, "hanako", "hanako-password")

	// 誤ったパスワードは 401
	if status, _ := doJSON(t, http.MethodPost, "/login", "", map[string]string{
		"memberId": "taro", "password": "wrong",
	}); status != http.StatusUnauthorized {
		t.Errorf("誤パスワード: status = %d, want 401", status)
	}

	// トークンなしのアクセスは 401
	if status, _ := doJSON(t, http.MethodGet, "/members", "", nil); status != http.StatusUnauthorized {
		t.Errorf("トークンなし: status = %d, want 401", status)
	}

	// --- メンバー一覧 ---
	status, body := doJSON(t, http.MethodGet, "/members", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("members status = %d, body = %s", status, body)
	}
	var membersRes struct {
		Members []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"members"`
	}
	if err := json.Unmarshal(body, &membersRes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(membersRes.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(membersRes.Members))
	}

	// --- 支出登録 (ユーザー提示の例) ---
	day := testMonth + "-15"
	status, body = doJSON(t, http.MethodPost, "/expenses", taro, map[string]any{
		"paidBy": taroID, "amountYen": 20000, "description": "家賃(一部)", "date": day,
	})
	if status != http.StatusCreated {
		t.Fatalf("expense status = %d, body = %s", status, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if status, body = doJSON(t, http.MethodPost, "/expenses", hanako, map[string]any{
		"paidBy": hanakoID, "amountYen": 20000, "description": "食費", "date": day,
	}); status != http.StatusCreated {
		t.Fatalf("expense status = %d, body = %s", status, body)
	}

	// --- 一覧 (DynamoDBから読めること) ---
	status, body = doJSON(t, http.MethodGet, "/expenses?month="+testMonth, hanako, nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", status, body)
	}
	var list struct {
		Expenses []struct {
			ID string `json:"id"`
		} `json:"expenses"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Expenses) != 2 {
		t.Fatalf("expenses = %d, want 2 (body = %s)", len(list.Expenses), body)
	}

	// --- 収入が揃う前の精算は 409 ---
	if status, _ = doJSON(t, http.MethodGet, "/months/"+testMonth+"/settlement", taro, nil); status != http.StatusConflict {
		t.Errorf("settlement(収入未入力) status = %d, want 409", status)
	}

	// --- 収入入力 ---
	if status, body = doJSON(t, http.MethodPut, "/months/"+testMonth+"/incomes/"+taroID, taro, map[string]any{
		"amountYen": 100000,
	}); status != http.StatusOK {
		t.Fatalf("income status = %d, body = %s", status, body)
	}
	if status, body = doJSON(t, http.MethodPut, "/months/"+testMonth+"/incomes/"+hanakoID, hanako, map[string]any{
		"amountYen": 50000,
	}); status != http.StatusOK {
		t.Fatalf("income status = %d, body = %s", status, body)
	}

	// --- 精算: 比重1:1 → 太郎が花子に25000円 ---
	status, body = doJSON(t, http.MethodGet, "/months/"+testMonth+"/settlement", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("settlement status = %d, body = %s", status, body)
	}
	var settlement struct {
		TotalExpenseYen int64 `json:"totalExpenseYen"`
		Members         []struct {
			ID            string `json:"id"`
			DisposableYen int64  `json:"disposableYen"`
		} `json:"members"`
		Transfer *struct {
			From      string `json:"from"`
			To        string `json:"to"`
			AmountYen int64  `json:"amountYen"`
		} `json:"transfer"`
	}
	if err := json.Unmarshal(body, &settlement); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if settlement.TotalExpenseYen != 40000 {
		t.Errorf("totalExpenseYen = %d, want 40000", settlement.TotalExpenseYen)
	}
	if settlement.Transfer == nil ||
		settlement.Transfer.From != taroID || settlement.Transfer.To != hanakoID || settlement.Transfer.AmountYen != 25000 {
		t.Errorf("transfer = %+v, want taro→hanako 25000", settlement.Transfer)
	}
	for _, m := range settlement.Members {
		if m.DisposableYen != 55000 {
			t.Errorf("%s disposable = %d, want 55000", m.ID, m.DisposableYen)
		}
	}

	// --- 支出削除 ---
	if status, _ = doJSON(t, http.MethodDelete, "/expenses/"+created.ID, taro, nil); status != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", status)
	}
	if status, _ = doJSON(t, http.MethodDelete, "/expenses/"+created.ID, taro, nil); status != http.StatusNotFound {
		t.Errorf("再削除 status = %d, want 404", status)
	}
}

func TestWeightSettings(t *testing.T) {
	waitForHealthy(t)
	taro, taroID := login(t, "taro", "taro-password")
	_, hanakoID := login(t, "hanako", "hanako-password")

	// 比重の更新が永続化されること
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

	// 後続テストに影響しないよう1:1へ戻す
	if status, _ = doJSON(t, http.MethodPut, "/settings/weight", taro, map[string]any{
		"weights": map[string]int64{taroID: 1, hanakoID: 1},
	}); status != http.StatusOK {
		t.Fatalf("weight reset status = %d", status)
	}
}

func TestValidationErrors(t *testing.T) {
	waitForHealthy(t)
	taro, taroID := login(t, "taro", "taro-password")

	cases := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{"負の金額", http.MethodPost, "/expenses", map[string]any{"paidBy": taroID, "amountYen": -1, "date": testMonth + "-01"}, 400},
		{"不明メンバー", http.MethodPost, "/expenses", map[string]any{"paidBy": "nobody", "amountYen": 100, "date": testMonth + "-01"}, 400},
		{"月形式不正", http.MethodGet, "/expenses?month=bad", nil, 400},
		{"収入の月形式不正", http.MethodPut, "/months/bad/incomes/" + taroID, map[string]any{"amountYen": 1}, 400},
		{"存在しない支出", http.MethodDelete, fmt.Sprintf("/expenses/%s_missing", testMonth), nil, 404},
	}
	for _, tt := range cases {
		if status, body := doJSON(t, tt.method, tt.path, taro, tt.body); status != tt.wantStatus {
			t.Errorf("%s: status = %d, want %d (body = %s)", tt.name, status, tt.wantStatus, body)
		}
	}
}

func TestClosingDaySettlement(t *testing.T) {
	waitForHealthy(t)
	taro, taroID := login(t, "taro", "taro-password")
	hanako, hanakoID := login(t, "hanako", "hanako-password")

	// 締め日はグローバル設定。他テストへ影響しないよう終了時に既定(1)へ戻す。
	defer func() {
		if status, body := doJSON(t, http.MethodPut, "/settings/closing-day", taro, map[string]any{"closingDay": 1}); status != http.StatusOK {
			t.Fatalf("closing-day reset status = %d, body = %s", status, body)
		}
	}()

	// 締め日=15 に設定（6/15〜7/14 → 7月分）。他テストの月(2030年台)と衝突しない2035年で検証する。
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

	// 不正値は 400
	if status, _ := doJSON(t, http.MethodPut, "/settings/closing-day", taro, map[string]any{"closingDay": 32}); status != http.StatusBadRequest {
		t.Errorf("closing-day=32 status = %d, want 400", status)
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
	status, body = doJSON(t, http.MethodGet, "/expenses?month=2035-07", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", status, body)
	}
	var list struct {
		Expenses []struct {
			Date string `json:"date"`
		} `json:"expenses"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Expenses) != 2 {
		t.Fatalf("7月の支出件数 = %d, want 2 (body = %s)", len(list.Expenses), body)
	}

	// 8月には 7/15 分のみ
	status, body = doJSON(t, http.MethodGet, "/expenses?month=2035-08", taro, nil)
	if status != http.StatusOK {
		t.Fatalf("list(8月) status = %d, body = %s", status, body)
	}
	list.Expenses = nil
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Expenses) != 1 || list.Expenses[0].Date != "2035-07-15" {
		t.Errorf("8月の支出 = %+v, want [2035-07-15]", list.Expenses)
	}

	// 精算の合計支出は 2000+4000 = 6000
	if status, _ := doJSON(t, http.MethodPut, "/months/2035-07/incomes/"+taroID, taro, map[string]any{"amountYen": 100000}); status != http.StatusOK {
		t.Fatalf("income(taro) status = %d", status)
	}
	if status, _ := doJSON(t, http.MethodPut, "/months/2035-07/incomes/"+hanakoID, hanako, map[string]any{"amountYen": 100000}); status != http.StatusOK {
		t.Fatalf("income(hanako) status = %d", status)
	}
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
