package domain

import (
	"fmt"
	"strings"
	"time"
)

// ExpenseID は共有支出の識別子。
// "<YYYY-MM>_<suffix>" 形式で対象年月を内包し、IDだけで格納先の月を特定できる。
type ExpenseID string

// NewExpenseID は対象年月とサフィックスから ExpenseID を生成する。
func NewExpenseID(month YearMonth, suffix string) ExpenseID {
	return ExpenseID(month.String() + "_" + suffix)
}

// Month は ExpenseID から対象年月を取り出す。
func (id ExpenseID) Month() (YearMonth, error) {
	s, _, ok := strings.Cut(string(id), "_")
	if !ok {
		return YearMonth{}, fmt.Errorf("%w: 支出IDの形式が不正です: %q", ErrValidation, id)
	}
	return ParseYearMonth(s)
}

// Expense はクライアントの共有支出を表すエンティティ。
type Expense struct {
	ID          ExpenseID
	PaidBy      MemberID // 立て替えたメンバー
	Amount      Money    // 正の金額（円）
	Description string
	Date        time.Time // 支出日（日付のみ有効）
	CreatedAt   time.Time
}

// NewExpense は共有支出を生成する。IDは支出日の年月とサフィックスから採番される。
func NewExpense(suffix string, paidBy MemberID, amount Money, description string, date time.Time, now time.Time) (Expense, error) {
	if suffix == "" {
		return Expense{}, fmt.Errorf("%w: 支出IDサフィックスは必須です", ErrValidation)
	}
	if paidBy == "" {
		return Expense{}, fmt.Errorf("%w: 支払者は必須です", ErrValidation)
	}
	if amount <= 0 {
		return Expense{}, fmt.Errorf("%w: 支出金額は正の整数（円）で指定してください: %d", ErrValidation, amount)
	}
	if date.IsZero() {
		return Expense{}, fmt.Errorf("%w: 支出日は必須です", ErrValidation)
	}
	return Expense{
		ID:          NewExpenseID(YearMonthOf(date), suffix),
		PaidBy:      paidBy,
		Amount:      amount,
		Description: strings.TrimSpace(description),
		Date:        date,
		CreatedAt:   now,
	}, nil
}

// Month は支出の対象年月を返す。
func (e Expense) Month() YearMonth { return YearMonthOf(e.Date) }
