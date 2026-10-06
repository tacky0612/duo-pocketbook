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

// EstimateSalaries は対象月の給与が未入力のメンバーについて、前月の給与実績で補った
// 対象月の給与一覧を返す（概算精算用）。estimated は前月実績で補ったメンバーの ID。
//
// 対象月に入力済みのメンバーはその値をそのまま使う。前月も未入力のメンバーは補わないため、
// その場合は精算計算で ErrIncomeNotReady となる。previous に前月以外の給与が含まれていても無視する。
func EstimateSalaries(month YearMonth, couple Couple, current, previous []Salary) (salaries []Salary, estimated []MemberID) {
	prevMonth := month.Prev()
	for _, m := range couple.Members() {
		if s, ok := findSalary(current, month, m.ID); ok {
			salaries = append(salaries, s)
			continue
		}
		if s, ok := findSalary(previous, prevMonth, m.ID); ok {
			salaries = append(salaries, Salary{Month: month, MemberID: m.ID, Amount: s.Amount})
			estimated = append(estimated, m.ID)
		}
	}
	return salaries, estimated
}

// findSalary は list から指定月・メンバーの給与を探す。
func findSalary(list []Salary, month YearMonth, memberID MemberID) (Salary, bool) {
	for _, s := range list {
		if s.Month == month && s.MemberID == memberID {
			return s, true
		}
	}
	return Salary{}, false
}
