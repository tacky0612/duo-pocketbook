package web_test

// Web 層テストの共通ヘルパー（インメモリ構成のテストサーバー・リクエスト送信・ログイン）。

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
