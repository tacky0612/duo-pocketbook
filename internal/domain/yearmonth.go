package domain

import (
	"fmt"
	"time"
)

// YearMonth は対象年月（例: 2026-07）を表す値オブジェクト。
type YearMonth struct {
	year  int
	month time.Month
}

// NewYearMonth は年と月から YearMonth を生成する。
func NewYearMonth(year int, month time.Month) (YearMonth, error) {
	if year < 2000 || year > 2999 {
		return YearMonth{}, fmt.Errorf("%w: 年は2000〜2999の範囲で指定してください: %d", ErrValidation, year)
	}
	if month < time.January || month > time.December {
		return YearMonth{}, fmt.Errorf("%w: 月は1〜12の範囲で指定してください: %d", ErrValidation, month)
	}
	return YearMonth{year: year, month: month}, nil
}

// ParseYearMonth は "2006-01" 形式の文字列をパースする。
func ParseYearMonth(s string) (YearMonth, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return YearMonth{}, fmt.Errorf("%w: 年月は YYYY-MM 形式で指定してください: %q", ErrValidation, s)
	}
	return NewYearMonth(t.Year(), t.Month())
}

// YearMonthOf は日付から YearMonth を導出する。
func YearMonthOf(t time.Time) YearMonth {
	return YearMonth{year: t.Year(), month: t.Month()}
}

// Year は年を返す。
func (ym YearMonth) Year() int { return ym.year }

// Month は月を返す。
func (ym YearMonth) Month() time.Month { return ym.month }

// String は "2006-01" 形式の文字列を返す。
func (ym YearMonth) String() string {
	return fmt.Sprintf("%04d-%02d", ym.year, int(ym.month))
}

// IsZero は未初期化かどうかを返す。
func (ym YearMonth) IsZero() bool { return ym.year == 0 }
