package domain

import "fmt"

// Salary はあるメンバーのある月の給与（毎月発生する基本の収入）を表す。
// メンバーごと・月ごとに1件のみで、精算の可否判定に用いる。
type Salary struct {
	Month    YearMonth
	MemberID MemberID
	Amount   Money // 0以上の金額（円）
}

// NewSalary は給与を生成する。
func NewSalary(month YearMonth, memberID MemberID, amount Money) (Salary, error) {
	if month.IsZero() {
		return Salary{}, fmt.Errorf("%w: 対象年月は必須です", ErrValidation)
	}
	if memberID == "" {
		return Salary{}, fmt.Errorf("%w: メンバーIDは必須です", ErrValidation)
	}
	if amount < 0 {
		return Salary{}, fmt.Errorf("%w: 給与金額は0以上の整数（円）で指定してください: %d", ErrValidation, amount)
	}
	return Salary{Month: month, MemberID: memberID, Amount: amount}, nil
}
