package domain

import "fmt"

// MonthlyIncome はあるメンバーのある月の収入を表す。
type MonthlyIncome struct {
	Month    YearMonth
	MemberID MemberID
	Amount   Money // 0以上の金額（円）
}

// NewMonthlyIncome は月次収入を生成する。
func NewMonthlyIncome(month YearMonth, memberID MemberID, amount Money) (MonthlyIncome, error) {
	if month.IsZero() {
		return MonthlyIncome{}, fmt.Errorf("%w: 対象年月は必須です", ErrValidation)
	}
	if memberID == "" {
		return MonthlyIncome{}, fmt.Errorf("%w: メンバーIDは必須です", ErrValidation)
	}
	if amount < 0 {
		return MonthlyIncome{}, fmt.Errorf("%w: 収入金額は0以上の整数（円）で指定してください: %d", ErrValidation, amount)
	}
	return MonthlyIncome{Month: month, MemberID: memberID, Amount: amount}, nil
}
