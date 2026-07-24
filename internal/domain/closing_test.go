package domain_test

import (
	"testing"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestClosingDaySettlementMonth(t *testing.T) {
	cases := []struct {
		name    string
		day     int
		date    time.Time
		wantYYM string
	}{
		// 締め日=1（デフォルト）は暦月どおり
		{"default 月初", 1, date(2026, 7, 1), "2026-07"},
		{"default 月末", 1, date(2026, 7, 31), "2026-07"},
		// 締め日=15: (前月)15日〜(当月)14日 を当月分に計上
		{"15締め 当月14日は当月", 15, date(2026, 7, 14), "2026-07"},
		{"15締め 当月15日は翌月", 15, date(2026, 7, 15), "2026-08"},
		{"15締め 前月15日は当月", 15, date(2026, 6, 15), "2026-07"},
		{"15締め 前月14日は前月", 15, date(2026, 6, 14), "2026-06"},
		// 年跨ぎ
		{"15締め 12月15日は翌年1月", 15, date(2026, 12, 15), "2027-01"},
		// 月末丸め: 2月に31日締め → 実効28日
		{"31締め 2月28日は3月(丸め)", 31, date(2026, 2, 28), "2026-03"},
		{"31締め 2月27日は2月", 31, date(2026, 2, 27), "2026-02"},
		{"30締め 2月28日は3月(丸め)", 30, date(2026, 2, 28), "2026-03"},
		// 31日ある月は丸めなし
		{"31締め 1月31日は2月", 31, date(2026, 1, 31), "2026-02"},
		{"31締め 1月30日は1月", 31, date(2026, 1, 30), "2026-01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cd, err := domain.NewClosingDay(tc.day)
			if err != nil {
				t.Fatalf("NewClosingDay(%d): %v", tc.day, err)
			}
			got := cd.SettlementMonth(tc.date).String()
			if got != tc.wantYYM {
				t.Errorf("SettlementMonth(%s, 締め日%d) = %s, want %s",
					tc.date.Format("2006-01-02"), tc.day, got, tc.wantYYM)
			}
		})
	}
}

func TestNewClosingDayValidation(t *testing.T) {
	for _, d := range []int{0, -1, 32, 100} {
		if _, err := domain.NewClosingDay(d); err == nil {
			t.Errorf("NewClosingDay(%d) はエラーになるべき", d)
		}
	}
	for _, d := range []int{1, 15, 28, 31} {
		if _, err := domain.NewClosingDay(d); err != nil {
			t.Errorf("NewClosingDay(%d) は成功すべき: %v", d, err)
		}
	}
}
