package domain

import (
	"fmt"
	"strings"
	"time"
)

// RecurringExpenseID は固定費（毎月発生する共有支出）の識別子。
type RecurringExpenseID string

// RecurringExpense は毎月定額で発生する共有支出（家賃・光熱費など）を表すエンティティ。
// 特定の月には紐づかず、精算時に対象月の共有支出として実体化される。
type RecurringExpense struct {
	ID          RecurringExpenseID
	PaidBy      MemberID // 毎月立て替えるメンバー
	Amount      Money    // 正の金額（円）
	Description string
}

// NewRecurringExpense は固定費を生成する。
func NewRecurringExpense(id string, paidBy MemberID, amount Money, description string) (RecurringExpense, error) {
	if id == "" {
		return RecurringExpense{}, fmt.Errorf("%w: 固定費IDは必須です", ErrValidation)
	}
	if paidBy == "" {
		return RecurringExpense{}, fmt.Errorf("%w: 支払者は必須です", ErrValidation)
	}
	if amount <= 0 {
		return RecurringExpense{}, fmt.Errorf("%w: 固定費の金額は正の整数（円）で指定してください: %d", ErrValidation, amount)
	}
	if strings.TrimSpace(description) == "" {
		return RecurringExpense{}, fmt.Errorf("%w: 固定費の内容は必須です", ErrValidation)
	}
	return RecurringExpense{
		ID:          RecurringExpenseID(id),
		PaidBy:      paidBy,
		Amount:      amount,
		Description: strings.TrimSpace(description),
	}, nil
}

// AsExpenseFor は固定費を対象月の共有支出として実体化する。
// 精算計算では通常の共有支出と同じように扱われる。
func (r RecurringExpense) AsExpenseFor(month YearMonth) Expense {
	date := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	return Expense{
		ID:          ExpenseID(month.String() + "_recurring-" + string(r.ID)),
		PaidBy:      r.PaidBy,
		Amount:      r.Amount,
		Description: r.Description,
		Date:        date,
		CreatedAt:   date,
	}
}
