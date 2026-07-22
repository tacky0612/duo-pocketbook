package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// RecurringExpenseUsecase は固定費（毎月発生する共有支出）に関するユースケース。
type RecurringExpenseUsecase struct {
	couple    domain.Couple
	recurring RecurringExpenseRepository
}

// NewRecurringExpenseUsecase は RecurringExpenseUsecase を生成する。
func NewRecurringExpenseUsecase(couple domain.Couple, recurring RecurringExpenseRepository) *RecurringExpenseUsecase {
	return &RecurringExpenseUsecase{couple: couple, recurring: recurring}
}

// RegisterRecurringExpenseInput は固定費登録の入力。
type RegisterRecurringExpenseInput struct {
	PaidBy      domain.MemberID
	AmountYen   int64
	Description string
}

// Register は固定費を登録する。
func (u *RecurringExpenseUsecase) Register(ctx context.Context, in RegisterRecurringExpenseInput) (domain.RecurringExpense, error) {
	if !u.couple.Contains(in.PaidBy) {
		return domain.RecurringExpense{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.PaidBy)
	}
	e, err := domain.NewRecurringExpense(newIDSuffix(), in.PaidBy, domain.Money(in.AmountYen), in.Description)
	if err != nil {
		return domain.RecurringExpense{}, err
	}
	if err := u.recurring.Save(ctx, e); err != nil {
		return domain.RecurringExpense{}, fmt.Errorf("固定費の保存に失敗しました: %w", err)
	}
	return e, nil
}

// Update は既存の固定費の内容を更新する（IDは維持）。
func (u *RecurringExpenseUsecase) Update(ctx context.Context, id domain.RecurringExpenseID, in RegisterRecurringExpenseInput) (domain.RecurringExpense, error) {
	if !u.couple.Contains(in.PaidBy) {
		return domain.RecurringExpense{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.PaidBy)
	}
	if _, err := u.recurring.FindByID(ctx, id); err != nil {
		return domain.RecurringExpense{}, err
	}
	e, err := domain.NewRecurringExpense(string(id), in.PaidBy, domain.Money(in.AmountYen), in.Description)
	if err != nil {
		return domain.RecurringExpense{}, err
	}
	if err := u.recurring.Save(ctx, e); err != nil {
		return domain.RecurringExpense{}, fmt.Errorf("固定費の更新に失敗しました: %w", err)
	}
	return e, nil
}

// List は登録済みの固定費を内容の昇順で返す。
func (u *RecurringExpenseUsecase) List(ctx context.Context) ([]domain.RecurringExpense, error) {
	list, err := u.recurring.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("固定費の取得に失敗しました: %w", err)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Description < list[j].Description
	})
	return list, nil
}

// Delete は固定費を削除する。
func (u *RecurringExpenseUsecase) Delete(ctx context.Context, id domain.RecurringExpenseID) error {
	if _, err := u.recurring.FindByID(ctx, id); err != nil {
		return err
	}
	if err := u.recurring.Delete(ctx, id); err != nil {
		return fmt.Errorf("固定費の削除に失敗しました: %w", err)
	}
	return nil
}
