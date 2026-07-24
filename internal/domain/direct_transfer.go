package domain

import (
	"fmt"
	"strings"
)

// DirectTransferID は立替精算の識別子。
// 単発（特定月のみ）は "<YYYY-MM>_<suffix>" 形式で対象年月を内包する。
// 継続（毎月）は "dtr_<suffix>" 形式で、先頭の予約プレフィックスで判別する。
type DirectTransferID string

// directTransferRecurringPrefix は継続の立替精算IDの先頭に付く予約語。
const directTransferRecurringPrefix = "dtr"

// NewOneOffDirectTransferID は対象年月とサフィックスから単発の立替精算IDを生成する。
func NewOneOffDirectTransferID(month YearMonth, suffix string) DirectTransferID {
	return DirectTransferID(month.String() + "_" + suffix)
}

// NewRecurringDirectTransferID はサフィックスから継続の立替精算IDを生成する。
func NewRecurringDirectTransferID(suffix string) DirectTransferID {
	return DirectTransferID(directTransferRecurringPrefix + "_" + suffix)
}

// IsRecurring はIDが継続（毎月）の立替精算を指すかを返す。
func (id DirectTransferID) IsRecurring() bool {
	return strings.HasPrefix(string(id), directTransferRecurringPrefix+"_")
}

// Month は単発の立替精算IDから対象年月を取り出す。継続IDでは失敗する。
func (id DirectTransferID) Month() (YearMonth, error) {
	if id.IsRecurring() {
		return YearMonth{}, fmt.Errorf("%w: 継続の立替精算には対象月がありません: %q", ErrValidation, id)
	}
	s, _, ok := strings.Cut(string(id), "_")
	if !ok {
		return YearMonth{}, fmt.Errorf("%w: 立替精算IDの形式が不正です: %q", ErrValidation, id)
	}
	return ParseYearMonth(s)
}

// DirectTransfer は共有支出とは別に、一方のメンバーから他方へ直接渡す送金を表すエンティティ。
//
// 比重による按分には含めず、月次の振込額へ純額としてそのまま加算する。
// Month がゼロ値なら毎月継続（継続）、値ありならその精算月のみ有効（単発）。
type DirectTransfer struct {
	ID          DirectTransferID
	From        MemberID  // 送る人
	To          MemberID  // 受け取る人
	Amount      Money     // 正の金額（円）
	Description string    // 内容
	Month       YearMonth // ゼロ値なら毎月継続
}

// NewDirectTransfer は立替精算を生成する。month がゼロ値なら継続として扱う。
func NewDirectTransfer(id string, from, to MemberID, amount Money, description string, month YearMonth) (DirectTransfer, error) {
	if id == "" {
		return DirectTransfer{}, fmt.Errorf("%w: 立替精算IDは必須です", ErrValidation)
	}
	if from == "" || to == "" {
		return DirectTransfer{}, fmt.Errorf("%w: 送金元・送金先は必須です", ErrValidation)
	}
	if from == to {
		return DirectTransfer{}, fmt.Errorf("%w: 送金元と送金先が同一です: %s", ErrValidation, from)
	}
	if amount <= 0 {
		return DirectTransfer{}, fmt.Errorf("%w: 立替精算の金額は正の整数（円）で指定してください: %d", ErrValidation, amount)
	}
	if strings.TrimSpace(description) == "" {
		return DirectTransfer{}, fmt.Errorf("%w: 立替精算の内容は必須です", ErrValidation)
	}
	return DirectTransfer{
		ID:          DirectTransferID(id),
		From:        from,
		To:          to,
		Amount:      amount,
		Description: strings.TrimSpace(description),
		Month:       month,
	}, nil
}

// IsRecurring は毎月継続の立替精算かどうかを返す。
func (d DirectTransfer) IsRecurring() bool { return d.Month.IsZero() }
