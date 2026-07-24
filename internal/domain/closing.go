package domain

import (
	"fmt"
	"time"
)

// ClosingDay は精算の締め日（各精算期間の起算日）。
//
// 例: 締め日=15 の場合、毎月15日を起算日として「(前月)15日〜(当月)14日」に記録した
// 支出を当月分として計上する（6/15〜7/14 → 7月分）。
// 締め日=1（デフォルト）は暦月どおりでシフトしない。指定できるのは 1〜31。
// 29〜31 はその日が存在しない月（2月など）ではその月の末日に丸める。
type ClosingDay int

// DefaultClosingDay は未設定時の締め日（暦月どおり）。
const DefaultClosingDay ClosingDay = 1

// NewClosingDay は 1〜31 の締め日を生成する。
func NewClosingDay(d int) (ClosingDay, error) {
	if d < 1 || d > 31 {
		return 0, fmt.Errorf("%w: 締め日は1〜31の範囲で指定してください: %d", ErrValidation, d)
	}
	return ClosingDay(d), nil
}

// Int は締め日の整数値を返す。
func (cd ClosingDay) Int() int { return int(cd) }

// SettlementMonth は支出日 date が属する精算月を返す。
//
// 締め日=1（デフォルト）は暦月をそのまま返す。締め日 D>=2 のとき、その月の実効締め日
// effD = min(D, その月の日数) 以上の日は翌月分、それ未満の日は当月分になる。
func (cd ClosingDay) SettlementMonth(date time.Time) YearMonth {
	ym := YearMonthOf(date)
	if cd <= DefaultClosingDay {
		return ym
	}
	eff := int(cd)
	if dim := daysInMonth(date.Year(), date.Month()); eff > dim {
		eff = dim // 29〜31 が無い月は末日に丸める
	}
	if date.Day() >= eff {
		return ym.Next()
	}
	return ym
}

// daysInMonth は指定年月の日数を返す（翌月0日＝当月末日）。
func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
