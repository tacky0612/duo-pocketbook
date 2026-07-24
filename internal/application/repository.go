// Package application はユースケース（アプリケーションとしての動作）を定義する。
// 外部依存へのアクセスはリポジトリインターフェイス経由で行い、実装はインフラ層が提供する。
package application

import (
	"context"
	"errors"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ErrNotFound は対象のデータが存在しない場合にリポジトリが返すエラー。
var ErrNotFound = errors.New("not found")

// ExpenseRepository は共有支出の永続化を担う。
type ExpenseRepository interface {
	Save(ctx context.Context, e domain.Expense) error
	FindByID(ctx context.Context, id domain.ExpenseID) (domain.Expense, error)
	FindByMonth(ctx context.Context, month domain.YearMonth) ([]domain.Expense, error)
	Delete(ctx context.Context, id domain.ExpenseID) error
}

// IncomeRepository は月次収入の永続化を担う。
type IncomeRepository interface {
	Save(ctx context.Context, income domain.MonthlyIncome) error
	FindByMonth(ctx context.Context, month domain.YearMonth) ([]domain.MonthlyIncome, error)
}

// SettlementStatusRepository は月ごとの精算済みフラグの永続化を担う。
type SettlementStatusRepository interface {
	IsSettled(ctx context.Context, month domain.YearMonth) (bool, error)
	SetSettled(ctx context.Context, month domain.YearMonth, settled bool) error
}

// RecurringExpenseRepository は固定費（毎月発生する共有支出）の永続化を担う。
type RecurringExpenseRepository interface {
	Save(ctx context.Context, e domain.RecurringExpense) error
	FindByID(ctx context.Context, id domain.RecurringExpenseID) (domain.RecurringExpense, error)
	FindAll(ctx context.Context) ([]domain.RecurringExpense, error)
	Delete(ctx context.Context, id domain.RecurringExpenseID) error
}

// MemberProfile はメンバーごとの上書き可能なプロフィール（表示名・カラー）。
// 未設定の項目は空文字になる。
type MemberProfile struct {
	Name  string
	Color string // "#RRGGBB" 形式
}

// Account はログイン資格情報と不変の AccountID を持つアカウント。
// ID（AccountID）はデータのキー・JWT subject として使う不変値。
// LoginID はログインに使う可変のユーザー名。
type Account struct {
	ID           domain.MemberID // 不変の AccountID（例: acct_xxxx）
	Slot         int             // env 設定スロット(0/1)。プロビジョニングの安定リンク
	LoginID      string          // 可変のログインID
	PasswordHash string          // bcrypt ハッシュ
}

// AccountRepository はアカウント（資格情報）の永続化を担う。
type AccountRepository interface {
	List(ctx context.Context) ([]Account, error)
	Save(ctx context.Context, a Account) error
}

// SettingsRepository はアプリケーション設定（精算比重・プロフィール）の永続化を担う。
type SettingsRepository interface {
	// GetWeight は設定済みの比重を返す。未設定の場合は ok=false を返す。
	GetWeight(ctx context.Context) (weight domain.Weight, ok bool, err error)
	SaveWeight(ctx context.Context, weight domain.Weight) error
	// GetMemberProfiles はメンバーIDごとのプロフィール上書き設定を返す（未設定分は含まれない）。
	GetMemberProfiles(ctx context.Context) (map[domain.MemberID]MemberProfile, error)
	SaveMemberName(ctx context.Context, id domain.MemberID, name string) error
	SaveMemberColor(ctx context.Context, id domain.MemberID, color string) error
	// GetClosingDay は設定済みの締め日を返す。未設定の場合は ok=false。
	GetClosingDay(ctx context.Context) (day domain.ClosingDay, ok bool, err error)
	SaveClosingDay(ctx context.Context, day domain.ClosingDay) error
}
