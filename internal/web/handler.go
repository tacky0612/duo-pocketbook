// Package web はAPIインターフェイス（HTTPハンドラ・ルーティング・認証）を定義する。
// リクエスト/レスポンスの変換のみを担い、業務ロジックはアプリケーション層に委譲する。
//
// ハンドラはリソースごとに handler_<resource>.go へ分け、各ファイルにそのリソースの DTO・
// リクエスト/レスポンス型・ハンドラ・swag 注釈をまとめる（swag 注釈と DTO が api/openapi.yaml の生成元）。
// 共通のレスポンス・エラー変換は response.go に置く。
package web

import (
	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// Handler はAPIハンドラ群。
type Handler struct {
	couple     domain.Couple
	auth       *Authenticator
	account    *application.AccountUsecase
	expenses   *application.ExpenseUsecase
	settlement *application.SettlementUsecase
	settings   *application.SettingsUsecase
	recurring  *application.RecurringExpenseUsecase
	direct     *application.DirectTransferUsecase
	income     *application.IncomeUsecase
	reserve    *application.ReservationUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	couple domain.Couple,
	auth *Authenticator,
	account *application.AccountUsecase,
	expenses *application.ExpenseUsecase,
	settlement *application.SettlementUsecase,
	settings *application.SettingsUsecase,
	recurring *application.RecurringExpenseUsecase,
	direct *application.DirectTransferUsecase,
	income *application.IncomeUsecase,
	reserve *application.ReservationUsecase,
) *Handler {
	return &Handler{
		couple:     couple,
		auth:       auth,
		account:    account,
		expenses:   expenses,
		settlement: settlement,
		settings:   settings,
		recurring:  recurring,
		direct:     direct,
		income:     income,
		reserve:    reserve,
	}
}
