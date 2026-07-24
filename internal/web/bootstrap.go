package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/config"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
	dynamoinfra "github.com/tacky0612/duo-pocketbook/internal/infrastructure/dynamodb"
	"github.com/tacky0612/duo-pocketbook/internal/infrastructure/memory"
)

// BuildHandler は設定からリポジトリ・ユースケース・ルーターを組み立てる。
// TABLE_NAME が設定されていれば DynamoDB、なければインメモリ実装を使う。
func BuildHandler(ctx context.Context, cfg config.Config, opt RouterOption) (http.Handler, error) {
	var (
		expenseRepo   application.ExpenseRepository
		incomeRepo    application.IncomeRepository
		recurringRepo application.RecurringExpenseRepository
		directRepo    application.DirectTransferRepository
		settings      application.SettingsRepository
		statusRepo    application.SettlementStatusRepository
		accountRepo   application.AccountRepository
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
		expenseRepo, incomeRepo, recurringRepo, directRepo, settings, statusRepo, accountRepo =
			repos.Expenses, repos.Incomes, repos.Recurring, repos.Direct, repos.Settings, repos.Status, repos.Accounts
	} else {
		expenseRepo = memory.NewExpenseRepository()
		incomeRepo = memory.NewIncomeRepository()
		recurringRepo = memory.NewRecurringExpenseRepository()
		directRepo = memory.NewDirectTransferRepository()
		settings = memory.NewSettingsRepository()
		statusRepo = memory.NewSettlementStatusRepository()
		accountRepo = memory.NewAccountRepository()
	}

	// アカウント（不変の AccountID）をプロビジョニングし、その ID で Couple を構築する。
	// env の ACCOUNTn_LOGINID / パスワードは初回の初期値（seed）としてのみ使う。
	seeds := accountSeeds(cfg)
	account := application.NewAccountUsecase(accountRepo, seeds, nil)
	members, err := account.Provision(ctx)
	if err != nil {
		return nil, fmt.Errorf("アカウントの初期化に失敗しました: %w", err)
	}
	couple, err := domain.NewCouple(members[0], members[1])
	if err != nil {
		return nil, fmt.Errorf("メンバー設定が不正です: %w", err)
	}

	auth := NewAuthenticator(cfg.JWTSecret, cfg.TokenTTL, couple, nil)
	handler := NewHandler(
		couple,
		auth,
		account,
		application.NewExpenseUsecase(couple, expenseRepo, settings, nil),
		application.NewSettlementUsecase(couple, expenseRepo, incomeRepo, recurringRepo, directRepo, settings, statusRepo),
		application.NewSettingsUsecase(couple, settings),
		application.NewRecurringExpenseUsecase(couple, recurringRepo),
		application.NewDirectTransferUsecase(couple, directRepo),
	)
	// 事前共有キー検証を有効化（設定時のみ）。
	opt.ClientKey = cfg.ClientKey
	return NewRouter(handler, auth, cfg.AllowedOrigins, opt), nil
}

// accountSeeds は config から各スロットの初期値（seed）を作る。
func accountSeeds(cfg config.Config) [2]application.AccountSeed {
	var seeds [2]application.AccountSeed
	for i, m := range cfg.Members {
		seeds[i] = application.AccountSeed{
			LoginID: string(m.Member.ID),
			Name:    m.Member.Name,
			Hash:    m.PasswordHash,
			Plain:   m.PasswordPlain,
		}
	}
	return seeds
}
