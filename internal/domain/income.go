package domain

import (
	"fmt"
	"strings"
)

// IncomeID は収入（給与とは別の、内容付きの追加収入）の識別子。
// 単発（特定月のみ）は "<YYYY-MM>_<suffix>" 形式で対象年月を内包する。
// 継続（毎月）は "inc_<suffix>" 形式で、先頭の予約プレフィックスで判別する。
type IncomeID string

// incomeRecurringPrefix は継続の収入IDの先頭に付く予約語。
const incomeRecurringPrefix = "inc"

// NewOneOffIncomeID は対象年月とサフィックスから単発の収入IDを生成する。
func NewOneOffIncomeID(month YearMonth, suffix string) IncomeID {
	return IncomeID(month.String() + "_" + suffix)
}

// NewRecurringIncomeID はサフィックスから継続の収入IDを生成する。
func NewRecurringIncomeID(suffix string) IncomeID {
	return IncomeID(incomeRecurringPrefix + "_" + suffix)
}

// IsRecurring はIDが継続（毎月）の収入を指すかを返す。
func (id IncomeID) IsRecurring() bool {
	return strings.HasPrefix(string(id), incomeRecurringPrefix+"_")
}

// Month は単発の収入IDから対象年月を取り出す。継続IDでは失敗する。
func (id IncomeID) Month() (YearMonth, error) {
	if id.IsRecurring() {
		return YearMonth{}, fmt.Errorf("%w: 継続の収入には対象月がありません: %q", ErrValidation, id)
	}
	s, _, ok := strings.Cut(string(id), "_")
	if !ok {
		return YearMonth{}, fmt.Errorf("%w: 収入IDの形式が不正です: %q", ErrValidation, id)
	}
	return ParseYearMonth(s)
}

// Income は給与とは別に、あるメンバーが得る内容付きの収入を表すエンティティ。
//
// 給与（Salary）と合算して精算の可処分所得へ反映する。日付は持たず、
// Month がゼロ値なら毎月継続（継続）、値ありならその精算月のみ有効（単発）。
type Income struct {
	ID          IncomeID
	MemberID    MemberID  // 収入を得るメンバー
	Amount      Money     // 正の金額（円）
	Description string    // 内容
	Month       YearMonth // ゼロ値なら毎月継続
}

// NewIncome は収入を生成する。month がゼロ値なら継続として扱う。
func NewIncome(id string, memberID MemberID, amount Money, description string, month YearMonth) (Income, error) {
	if id == "" {
		return Income{}, fmt.Errorf("%w: 収入IDは必須です", ErrValidation)
	}
	if memberID == "" {
		return Income{}, fmt.Errorf("%w: メンバーIDは必須です", ErrValidation)
	}
	if amount <= 0 {
		return Income{}, fmt.Errorf("%w: 収入の金額は正の整数（円）で指定してください: %d", ErrValidation, amount)
	}
	if strings.TrimSpace(description) == "" {
		return Income{}, fmt.Errorf("%w: 収入の内容は必須です", ErrValidation)
	}
	return Income{
		ID:          IncomeID(id),
		MemberID:    memberID,
		Amount:      amount,
		Description: strings.TrimSpace(description),
		Month:       month,
	}, nil
}

// IsRecurring は毎月継続の収入かどうかを返す。
func (i Income) IsRecurring() bool { return i.Month.IsZero() }
