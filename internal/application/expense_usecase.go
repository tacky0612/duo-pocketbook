package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ExpenseUsecase は共有支出に関するユースケース。
type ExpenseUsecase struct {
	couple   domain.Couple
	expenses ExpenseRepository
	settings SettingsRepository
	now      func() time.Time
}

// NewExpenseUsecase は ExpenseUsecase を生成する。
func NewExpenseUsecase(couple domain.Couple, expenses ExpenseRepository, settings SettingsRepository, now func() time.Time) *ExpenseUsecase {
	if now == nil {
		now = time.Now
	}
	return &ExpenseUsecase{couple: couple, expenses: expenses, settings: settings, now: now}
}

// RegisterExpenseInput は支出登録の入力。
type RegisterExpenseInput struct {
	PaidBy      domain.MemberID
	AmountYen   int64
	Description string
	Date        string // YYYY-MM-DD
}

// Register は共有支出を登録する。
func (u *ExpenseUsecase) Register(ctx context.Context, in RegisterExpenseInput) (domain.Expense, error) {
	if !u.couple.Contains(in.PaidBy) {
		return domain.Expense{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.PaidBy)
	}
	date, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return domain.Expense{}, fmt.Errorf("%w: 支出日は YYYY-MM-DD 形式で指定してください: %q", domain.ErrValidation, in.Date)
	}
	e, err := domain.NewExpense(newIDSuffix(), in.PaidBy, domain.Money(in.AmountYen), in.Description, date, u.now())
	if err != nil {
		return domain.Expense{}, err
	}
	if err := u.expenses.Save(ctx, e); err != nil {
		return domain.Expense{}, fmt.Errorf("支出の保存に失敗しました: %w", err)
	}
	return e, nil
}

// Update は既存の共有支出の内容を更新する。
// 日付の変更で対象月が変わった場合は、新しい月のIDへ移し替える（旧レコードは削除する）。
func (u *ExpenseUsecase) Update(ctx context.Context, id domain.ExpenseID, in RegisterExpenseInput) (domain.Expense, error) {
	if !u.couple.Contains(in.PaidBy) {
		return domain.Expense{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.PaidBy)
	}
	existing, err := u.expenses.FindByID(ctx, id)
	if err != nil {
		return domain.Expense{}, err
	}
	date, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return domain.Expense{}, fmt.Errorf("%w: 支出日は YYYY-MM-DD 形式で指定してください: %q", domain.ErrValidation, in.Date)
	}
	// 既存IDのサフィックスを引き継ぐ。対象月が同じなら同一IDのまま上書きになる。
	_, suffix, _ := strings.Cut(string(id), "_")
	updated, err := domain.NewExpense(suffix, in.PaidBy, domain.Money(in.AmountYen), in.Description, date, existing.CreatedAt)
	if err != nil {
		return domain.Expense{}, err
	}
	if err := u.expenses.Save(ctx, updated); err != nil {
		return domain.Expense{}, fmt.Errorf("支出の更新に失敗しました: %w", err)
	}
	// 月が変わってIDが変化した場合は旧レコードを削除する。
	if updated.ID != id {
		if err := u.expenses.Delete(ctx, id); err != nil {
			return domain.Expense{}, fmt.Errorf("旧支出の削除に失敗しました: %w", err)
		}
	}
	return updated, nil
}

// ListByMonth は対象精算月の共有支出を日付降順で返す。
// 締め日設定に応じて精算期間（暦月をまたぐ場合がある）で集計する。
func (u *ExpenseUsecase) ListByMonth(ctx context.Context, month string) ([]domain.Expense, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return nil, err
	}
	list, err := expensesForSettlementMonth(ctx, u.expenses, ym, closingDay)
	if err != nil {
		return nil, fmt.Errorf("支出の取得に失敗しました: %w", err)
	}
	sort.Slice(list, func(i, j int) bool {
		if !list[i].Date.Equal(list[j].Date) {
			return list[i].Date.After(list[j].Date)
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

// Delete は共有支出を削除する。クライアントどちらのメンバーでも削除できる。
func (u *ExpenseUsecase) Delete(ctx context.Context, id domain.ExpenseID) error {
	if _, err := id.Month(); err != nil {
		return err
	}
	if _, err := u.expenses.FindByID(ctx, id); err != nil {
		return err
	}
	if err := u.expenses.Delete(ctx, id); err != nil {
		return fmt.Errorf("支出の削除に失敗しました: %w", err)
	}
	return nil
}
