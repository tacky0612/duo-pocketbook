package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

func TestParseYearMonth(t *testing.T) {
	ym, err := domain.ParseYearMonth("2026-07")
	if err != nil {
		t.Fatalf("ParseYearMonth: %v", err)
	}
	if ym.String() != "2026-07" {
		t.Errorf("String() = %q, want %q", ym.String(), "2026-07")
	}

	for _, invalid := range []string{"", "2026", "2026-13", "2026/07", "26-07"} {
		if _, err := domain.ParseYearMonth(invalid); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("ParseYearMonth(%q) err = %v, want ErrValidation", invalid, err)
		}
	}
}

func TestExpenseIDMonth(t *testing.T) {
	ym, _ := domain.ParseYearMonth("2026-07")
	id := domain.NewExpenseID(ym, "abc123")
	if id != "2026-07_abc123" {
		t.Errorf("id = %q, want %q", id, "2026-07_abc123")
	}
	got, err := id.Month()
	if err != nil {
		t.Fatalf("Month: %v", err)
	}
	if got != ym {
		t.Errorf("Month() = %v, want %v", got, ym)
	}

	if _, err := domain.ExpenseID("invalid").Month(); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestNewExpenseValidation(t *testing.T) {
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	now := date

	if _, err := domain.NewExpense("s", "taro", 0, "d", date, now); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("金額0: err = %v, want ErrValidation", err)
	}
	if _, err := domain.NewExpense("s", "taro", -100, "d", date, now); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("負の金額: err = %v, want ErrValidation", err)
	}
	if _, err := domain.NewExpense("s", "", 100, "d", date, now); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("支払者なし: err = %v, want ErrValidation", err)
	}
	if _, err := domain.NewExpense("", "taro", 100, "d", date, now); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("サフィックスなし: err = %v, want ErrValidation", err)
	}

	e, err := domain.NewExpense("s", "taro", 100, "  食費  ", date, now)
	if err != nil {
		t.Fatalf("NewExpense: %v", err)
	}
	if e.Description != "食費" {
		t.Errorf("Description = %q, want %q", e.Description, "食費")
	}
	if e.Month().String() != "2026-07" {
		t.Errorf("Month() = %v, want 2026-07", e.Month())
	}
}

func TestNewCoupleValidation(t *testing.T) {
	a := domain.Member{ID: "taro", Name: "太郎"}
	if _, err := domain.NewCouple(a, a); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("同一ID: err = %v, want ErrValidation", err)
	}
	if _, err := domain.NewCouple(a, domain.Member{}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("空ID: err = %v, want ErrValidation", err)
	}

	c, err := domain.NewCouple(a, domain.Member{ID: "hanako", Name: "花子"})
	if err != nil {
		t.Fatalf("NewCouple: %v", err)
	}
	if !c.Contains("taro") || !c.Contains("hanako") || c.Contains("other") {
		t.Error("Contains の判定が不正")
	}
	other, ok := c.Other("taro")
	if !ok || other.ID != "hanako" {
		t.Errorf("Other(taro) = %v, %v", other, ok)
	}
}

func TestNewWeightValidation(t *testing.T) {
	if _, err := domain.NewWeight("taro", 0, "hanako", 1); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("比重0: err = %v, want ErrValidation", err)
	}
	if _, err := domain.NewWeight("taro", 1, "taro", 1); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("同一ID: err = %v, want ErrValidation", err)
	}
	w, err := domain.NewWeight("taro", 2, "hanako", 3)
	if err != nil {
		t.Fatalf("NewWeight: %v", err)
	}
	if v, ok := w.Of("taro"); !ok || v != 2 {
		t.Errorf("Of(taro) = %d, %v", v, ok)
	}
}

func TestNewSalaryValidation(t *testing.T) {
	ym, _ := domain.ParseYearMonth("2026-07")
	if _, err := domain.NewSalary(ym, "taro", -1); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("負の給与: err = %v, want ErrValidation", err)
	}
	if _, err := domain.NewSalary(domain.YearMonth{}, "taro", 100); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("年月なし: err = %v, want ErrValidation", err)
	}
	// 給与0円は有効（無収入の月もあり得る）
	if _, err := domain.NewSalary(ym, "taro", 0); err != nil {
		t.Errorf("給与0円: err = %v, want nil", err)
	}
}

func TestNewIncomeValidation(t *testing.T) {
	ym, _ := domain.ParseYearMonth("2026-07")
	id := string(domain.NewOneOffIncomeID(ym, "abc"))
	if _, err := domain.NewIncome(id, "taro", 0, "副業", ym); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("金額0: err = %v, want ErrValidation", err)
	}
	if _, err := domain.NewIncome(id, "taro", 100, "", ym); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("内容なし: err = %v, want ErrValidation", err)
	}
	if _, err := domain.NewIncome("", "taro", 100, "副業", ym); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("IDなし: err = %v, want ErrValidation", err)
	}
	// 継続（月ゼロ値）は IsRecurring=true
	rid := string(domain.NewRecurringIncomeID("abc"))
	inc, err := domain.NewIncome(rid, "taro", 100, "副業", domain.YearMonth{})
	if err != nil {
		t.Fatalf("NewIncome(継続): %v", err)
	}
	if !inc.IsRecurring() {
		t.Errorf("継続の収入が IsRecurring=false")
	}
}
