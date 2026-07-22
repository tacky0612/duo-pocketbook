package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/config"
	dynamoinfra "github.com/tacky0612/duo-pocketbook/internal/infrastructure/dynamodb"
	"github.com/tacky0612/duo-pocketbook/internal/infrastructure/memory"
)

// BuildHandler は設定からリポジトリ・ユースケース・ルーターを組み立てる。
// TABLE_NAME が設定されていれば DynamoDB、なければインメモリ実装を使う。
func BuildHandler(ctx context.Context, cfg config.Config, opt RouterOption) (http.Handler, error) {
	couple, err := cfg.Couple()
	if err != nil {
		return nil, fmt.Errorf("メンバー設定が不正です: %w", err)
	}

	var (
		expenseRepo   application.ExpenseRepository
		incomeRepo    application.IncomeRepository
		recurringRepo application.RecurringExpenseRepository
		settings      application.SettingsRepository
		statusRepo    application.SettlementStatusRepository
	)
	if cfg.TableName != "" {
		client, err := dynamoinfra.NewClient(ctx, cfg.DynamoEndpoint)
		if err != nil {
			return nil, err
		}
		// ローカル(DynamoDB Local)の場合のみテーブルを自動作成する。
		if cfg.DynamoEndpoint != "" {
			if err := dynamoinfra.EnsureTable(ctx, client, cfg.TableName); err != nil {
				return nil, err
			}
		}
		repos := dynamoinfra.NewRepositories(client, cfg.TableName)
		expenseRepo, incomeRepo, recurringRepo, settings, statusRepo =
			repos.Expenses, repos.Incomes, repos.Recurring, repos.Settings, repos.Status
	} else {
		expenseRepo = memory.NewExpenseRepository()
		incomeRepo = memory.NewIncomeRepository()
		recurringRepo = memory.NewRecurringExpenseRepository()
		settings = memory.NewSettingsRepository()
		statusRepo = memory.NewSettlementStatusRepository()
	}

	auth := NewAuthenticator(cfg, couple, nil)
	handler := NewHandler(
		couple,
		auth,
		application.NewExpenseUsecase(couple, expenseRepo, nil),
		application.NewSettlementUsecase(couple, expenseRepo, incomeRepo, recurringRepo, settings, statusRepo),
		application.NewSettingsUsecase(couple, settings),
		application.NewRecurringExpenseUsecase(couple, recurringRepo),
	)
	return NewRouter(handler, auth, cfg.AllowedOrigins, opt), nil
}
