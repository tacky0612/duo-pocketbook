package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
	"github.com/tacky0612/duo-pocketbook/internal/infrastructure/memory"
	"github.com/tacky0612/duo-pocketbook/internal/web"
)

type testAccount struct {
	AccountID string
	LoginID   string
	Password  string
}

func newTestServer(t *testing.T) (*httptest.Server, [2]testAccount) {
	t.Helper()

	seeds := [2]application.AccountSeed{
		{LoginID: "taro", Name: "太郎", Plain: "taro-pass"},
		{LoginID: "hanako", Name: "花子", Plain: "hanako-pass"},
	}
	accountRepo := memory.NewAccountRepository()
	n := 0
	idgen := func() string { n++; return fmt.Sprintf("acct_test_%d", n) }
	account := application.NewAccountUsecase(accountRepo, seeds, idgen)
	members, err := account.Provision(context.Background())
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	couple, err := domain.NewCouple(members[0], members[1])
	if err != nil {
		t.Fatalf("NewCouple: %v", err)
	}

	expenseRepo := memory.NewExpenseRepository()
	salaryRepo := memory.NewSalaryRepository()
	incomeRepo := memory.NewIncomeRepository()
	recurringRepo := memory.NewRecurringExpenseRepository()
	directRepo := memory.NewDirectTransferRepository()
	reserveRepo := memory.NewReservationRepository()
	settingsRepo := memory.NewSettingsRepository()
	snapshotRepo := memory.NewSettlementSnapshotRepository()

	auth := web.NewAuthenticator("test-secret", time.Hour, couple, nil)
	handler := web.NewHandler(
		couple,
		auth,
		account,
		application.NewExpenseUsecase(couple, expenseRepo, settingsRepo, snapshotRepo, reserveRepo, nil),
		application.NewSettlementUsecase(couple, expenseRepo, salaryRepo, incomeRepo, recurringRepo, directRepo, settingsRepo, snapshotRepo, nil),
		application.NewSettingsUsecase(couple, settingsRepo),
		application.NewRecurringExpenseUsecase(couple, recurringRepo),
		application.NewDirectTransferUsecase(couple, directRepo, snapshotRepo),
		application.NewIncomeUsecase(couple, incomeRepo, snapshotRepo, reserveRepo),
		application.NewReservationUsecase(couple, reserveRepo, expenseRepo, incomeRepo, settingsRepo, snapshotRepo, nil),
	)
	srv := httptest.NewServer(web.NewRouter(handler, auth, []string{"*"}, web.RouterOption{}))
	t.Cleanup(srv.Close)

	accounts := [2]testAccount{
		{AccountID: string(members[0].ID), LoginID: "taro", Password: "taro-pass"},
		{AccountID: string(members[1].ID), LoginID: "hanako", Password: "hanako-pass"},
	}
	return srv, accounts
}

func doJSON(t *testing.T, method, url, token string, body any) (*http.Response, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, out.Bytes()
}

func login(t *testing.T, srv *httptest.Server, loginID, password string) string {
	t.Helper()
	resp, body := doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{
		"memberId": loginID, "password": password,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", resp.StatusCode, body)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out.Token
}

func TestLogin(t *testing.T) {
	srv, acc := newTestServer(t)

	token := login(t, srv, acc[0].LoginID, acc[0].Password)
	if token == "" {
		t.Fatal("token が空")
	}

	// パスワード誤り
	resp, _ := doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{
		"memberId": acc[0].LoginID, "password": "wrong",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}

	// 不明なログインID
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{
		"memberId": "unknown", "password": "taro-pass",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthRequired(t *testing.T) {
	srv, _ := newTestServer(t)

	paths := []struct{ method, path string }{
		{http.MethodGet, "/members"},
		{http.MethodGet, "/account"},
		{http.MethodPost, "/expenses"},
		{http.MethodGet, "/expenses?month=2026-07"},
		{http.MethodGet, "/months/2026-07/settlement"},
		{http.MethodGet, "/settings/weight"},
	}
	for _, p := range paths {
		resp, _ := doJSON(t, p.method, srv.URL+p.path, "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", p.method, p.path, resp.StatusCode)
		}
	}

	resp, _ := doJSON(t, http.MethodGet, srv.URL+"/members", "invalid-token", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("不正トークン: status = %d, want 401", resp.StatusCode)
	}
}

func TestExpenseAndSettlementAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	taro, hanako := acc[0], acc[1]
	taroToken := login(t, srv, taro.LoginID, taro.Password)
	hanakoToken := login(t, srv, hanako.LoginID, hanako.Password)

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/expenses", taroToken, map[string]any{
		"paidBy": taro.AccountID, "amountYen": 20000, "description": "家賃(一部)", "date": "2026-07-01",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp, body = doJSON(t, http.MethodPost, srv.URL+"/expenses", hanakoToken, map[string]any{
		"paidBy": hanako.AccountID, "amountYen": 20000, "description": "食費", "date": "2026-07-05",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	resp, body = doJSON(t, http.MethodGet, srv.URL+"/expenses?month=2026-07", taroToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
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
		t.Fatalf("len(expenses) = %d, want 2", len(list.Expenses))
	}

	// 給与が揃う前の精算は 409
	resp, _ = doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement", taroToken, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}

	// 給与入力
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/months/2026-07/salaries/"+taro.AccountID, taroToken, map[string]any{
		"amountYen": 100000,
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/months/2026-07/salaries/"+hanako.AccountID, hanakoToken, map[string]any{
		"amountYen": 50000,
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}

	// 精算: 太郎→花子 25000円
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/months/2026-07/settlement", hanakoToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var settlement struct {
		TotalExpenseYen int64 `json:"totalExpenseYen"`
		Transfer        *struct {
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
		settlement.Transfer.From != taro.AccountID || settlement.Transfer.To != hanako.AccountID || settlement.Transfer.AmountYen != 25000 {
		t.Errorf("transfer = %+v, want %s→%s 25000", settlement.Transfer, taro.AccountID, hanako.AccountID)
	}

	// 支出削除
	resp, _ = doJSON(t, http.MethodDelete, srv.URL+"/expenses/"+created.ID, taroToken, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", resp.StatusCode)
	}
	resp, _ = doJSON(t, http.MethodDelete, srv.URL+"/expenses/"+created.ID, taroToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("再削除 status = %d, want 404", resp.StatusCode)
	}

	// バリデーションエラー
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/expenses", taroToken, map[string]any{
		"paidBy": taro.AccountID, "amountYen": -100, "date": "2026-07-01",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

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

func TestAccountAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	token := login(t, srv, acc[0].LoginID, acc[0].Password)

	// GET /account
	resp, body := doJSON(t, http.MethodGet, srv.URL+"/account", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var a struct {
		AccountID string `json:"accountId"`
		LoginID   string `json:"loginId"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal(body, &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.AccountID != acc[0].AccountID || a.LoginID != "taro" {
		t.Fatalf("account = %+v", a)
	}

	// ログインID変更
	resp, body = doJSON(t, http.MethodPut, srv.URL+"/account/login-id", token, map[string]string{"loginId": "taro2"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login-id status = %d, body = %s", resp.StatusCode, body)
	}
	// 旧IDはログイン不可・新IDはログイン可
	resp, _ = doJSON(t, http.MethodPost, srv.URL+"/login", "", map[string]string{"memberId": "taro", "password": "taro-pass"})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("旧ID login status = %d, want 401", resp.StatusCode)
	}
	_ = login(t, srv, "taro2", "taro-pass")

	// 他アカウントと重複するIDは 400
	resp, _ = doJSON(t, http.MethodPut, srv.URL+"/account/login-id", token, map[string]string{"loginId": "hanako"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("重複ID status = %d, want 400", resp.StatusCode)
	}

	// パスワード変更（現在PW誤り→400）
	resp, _ = doJSON(t, http.MethodPut, srv.URL+"/account/password", token, map[string]string{
		"currentPassword": "wrong", "newPassword": "newpassword1",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("現在PW誤り status = %d, want 400", resp.StatusCode)
	}
	// 現在PW正しい→204、新PWでログイン可
	resp, _ = doJSON(t, http.MethodPut, srv.URL+"/account/password", token, map[string]string{
		"currentPassword": "taro-pass", "newPassword": "newpassword1",
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("PW変更 status = %d, want 204", resp.StatusCode)
	}
	_ = login(t, srv, "taro2", "newpassword1")
}

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

func TestTokenExpiry(t *testing.T) {
	member := domain.Member{ID: "acct_x", Name: "太郎"}
	couple, err := domain.NewCouple(member, domain.Member{ID: "acct_y", Name: "花子"})
	if err != nil {
		t.Fatalf("NewCouple: %v", err)
	}

	current := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	auth := web.NewAuthenticator("test-secret", time.Hour, couple, func() time.Time { return current })

	token, _, err := auth.IssueToken(member.ID)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := auth.Verify(token); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	current = current.Add(2 * time.Hour)
	if _, err := auth.Verify(token); err == nil {
		t.Fatal("期限切れトークンが有効と判定された")
	}
}

func TestReservationAPI(t *testing.T) {
	srv, acc := newTestServer(t)
	taro, hanako := acc[0], acc[1]
	token := login(t, srv, taro.LoginID, taro.Password)

	type reservation struct {
		ID                 string   `json:"id"`
		Kind               string   `json:"kind"`
		Recurring          bool     `json:"recurring"`
		Month              string   `json:"month"`
		Status             string   `json:"status"`
		FulfilledIDs       []string `json:"fulfilledIds"`
		FulfilledAmountYen int64    `json:"fulfilledAmountYen"`
		EstimatedAmountYen int64    `json:"estimatedAmountYen"`
	}
	decode := func(body []byte, v any) {
		t.Helper()
		if err := json.Unmarshal(body, v); err != nil {
			t.Fatalf("unmarshal: %v (%s)", err, body)
		}
	}

	resp, body := doJSON(t, http.MethodPost, srv.URL+"/reservations", token, map[string]any{
		"kind": "expense", "memberId": taro.AccountID, "description": "電気代", "estimatedAmountYen": 8000, "month": "",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var power reservation
	decode(body, &power)
	if !power.Recurring || power.Status != "pending" || power.EstimatedAmountYen != 8000 {
		t.Errorf("created = %+v", power)
	}

	resp, body = doJSON(t, http.MethodPost, srv.URL+"/reservations", token, map[string]any{
		"kind": "income", "memberId": hanako.AccountID, "description": "賞与", "month": "2026-07",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	var bonus reservation
	decode(body, &bonus)

	// 不正な種別は 400。
	if resp, body = doJSON(t, http.MethodPost, srv.URL+"/reservations", token, map[string]any{
		"kind": "x", "memberId": taro.AccountID, "description": "x",
	}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid kind: status = %d, body = %s", resp.StatusCode, body)
	}

	var list struct {
		Month        string        `json:"month"`
		Reservations []reservation `json:"reservations"`
		PendingCount int           `json:"pendingCount"`
	}
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/reservations?month=2026-07", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, body)
	}
	decode(body, &list)
	if len(list.Reservations) != 2 || list.PendingCount != 2 {
		t.Fatalf("list = %+v", list)
	}

	// 支出の予約を入力 → 共有支出に reservationId 付きで現れる。
	resp, body = doJSON(t, http.MethodPost, srv.URL+"/reservations/"+power.ID+"/fulfill", token, map[string]any{
		"month": "2026-07", "amountYen": 7820, "date": "2026-07-25",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("fulfill status = %d, body = %s", resp.StatusCode, body)
	}
	var fulfilled reservation
	decode(body, &fulfilled)
	if fulfilled.Status != "fulfilled" || fulfilled.FulfilledAmountYen != 7820 || len(fulfilled.FulfilledIDs) != 1 {
		t.Errorf("fulfilled = %+v", fulfilled)
	}
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/expenses?month=2026-07", token, nil)
	var expenses struct {
		Expenses []struct {
			ID            string `json:"id"`
			ReservationID string `json:"reservationId"`
		} `json:"expenses"`
	}
	decode(body, &expenses)
	if len(expenses.Expenses) != 1 || expenses.Expenses[0].ReservationID != power.ID || expenses.Expenses[0].ID != fulfilled.FulfilledIDs[0] {
		t.Errorf("expenses = %+v", expenses)
	}

	// 今月だけの予約は「今月はなし」にできない（400）。毎月の予約はできる。
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/reservations/"+bonus.ID+"/skip", token, map[string]any{
		"month": "2026-07", "skipped": true,
	}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("one-off skip status = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, http.MethodPut, srv.URL+"/reservations/"+power.ID+"/skip", token, map[string]any{
		"month": "2026-08", "skipped": true,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("skip status = %d, body = %s", resp.StatusCode, body)
	}
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/reservations?month=2026-08&kind=expense", token, nil)
	list.Reservations = nil
	decode(body, &list)
	if len(list.Reservations) != 1 || list.Reservations[0].Status != "skipped" || list.PendingCount != 0 {
		t.Errorf("expense list (2026-08) = %+v", list)
	}
	resp, body = doJSON(t, http.MethodGet, srv.URL+"/reservations?month=2026-07&kind=income", token, nil)
	list.Reservations = nil
	decode(body, &list)
	if len(list.Reservations) != 1 || list.Reservations[0].Status != "pending" || list.PendingCount != 1 {
		t.Errorf("income list = %+v", list)
	}

	// 更新・削除。
	if resp, body = doJSON(t, http.MethodPut, srv.URL+"/reservations/"+power.ID, token, map[string]any{
		"memberId": hanako.AccountID, "description": "電気代", "estimatedAmountYen": 9000,
	}); resp.StatusCode != http.StatusOK {
		t.Errorf("update status = %d, body = %s", resp.StatusCode, body)
	}
	if resp, body = doJSON(t, http.MethodDelete, srv.URL+"/reservations/"+bonus.ID, token, nil); resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, body = %s", resp.StatusCode, body)
	}
	if resp, body = doJSON(t, http.MethodDelete, srv.URL+"/reservations/"+bonus.ID, token, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("delete again status = %d, body = %s", resp.StatusCode, body)
	}
}
