package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

const (
	husband = domain.MemberID("taro")
	wife    = domain.MemberID("hanako")
)

func testCouple(t *testing.T) domain.Couple {
	t.Helper()
	c, err := domain.NewCouple(
		domain.Member{ID: husband, Name: "太郎"},
		domain.Member{ID: wife, Name: "花子"},
	)
	if err != nil {
		t.Fatalf("NewCouple: %v", err)
	}
	return c
}

func testWeight(t *testing.T, wHusband, wWife int64) domain.Weight {
	t.Helper()
	w, err := domain.NewWeight(husband, wHusband, wife, wWife)
	if err != nil {
		t.Fatalf("NewWeight: %v", err)
	}
	return w
}

func testMonth(t *testing.T) domain.YearMonth {
	t.Helper()
	ym, err := domain.ParseYearMonth("2026-07")
	if err != nil {
		t.Fatalf("ParseYearMonth: %v", err)
	}
	return ym
}

func testSalary(t *testing.T, id domain.MemberID, amount domain.Money) domain.Salary {
	t.Helper()
	s, err := domain.NewSalary(testMonth(t), id, amount)
	if err != nil {
		t.Fatalf("NewSalary: %v", err)
	}
	return s
}

func testExpense(t *testing.T, suffix string, paidBy domain.MemberID, amount domain.Money) domain.Expense {
	t.Helper()
	date := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	e, err := domain.NewExpense(suffix, paidBy, amount, "テスト支出", date, date)
	if err != nil {
		t.Fatalf("NewExpense: %v", err)
	}
	return e
}

func TestCalculateSettlement(t *testing.T) {
	tests := []struct {
		name          string
		weightHusband int64
		weightWife    int64
		incomeHusband domain.Money
		incomeWife    domain.Money
		paidHusband   domain.Money // 0なら支出なし
		paidWife      domain.Money
		wantFrom      domain.MemberID
		wantTo        domain.MemberID
		wantAmount    domain.Money
		wantNil       bool // 精算不要
	}{
		{
			// ユーザー提示の例: 比重1:1、夫(収入10万・支出2万)、妻(収入5万・支出2万)
			// → 夫が妻に2.5万円振り込むと双方の可処分所得が5.5万円で等しくなる。
			name:          "比重1対1の基本例",
			weightHusband: 1, weightWife: 1,
			incomeHusband: 100_000, incomeWife: 50_000,
			paidHusband: 20_000, paidWife: 20_000,
			wantFrom: husband, wantTo: wife, wantAmount: 25_000,
		},
		{
			// 比重2:1: 夫の可処分所得が妻の2倍になるように精算する。
			// net夫=8万, net妻=3万 → t=(1*80000-2*30000)/3=6666.67→6667
			// 精算後: 夫73333, 妻36667 (約2:1)
			name:          "比重2対1と端数の四捨五入",
			weightHusband: 2, weightWife: 1,
			incomeHusband: 100_000, incomeWife: 50_000,
			paidHusband: 20_000, paidWife: 20_000,
			wantFrom: husband, wantTo: wife, wantAmount: 6_667,
		},
		{
			name:          "妻から夫への逆方向の精算",
			weightHusband: 1, weightWife: 1,
			incomeHusband: 50_000, incomeWife: 100_000,
			paidHusband: 30_000, paidWife: 0,
			wantFrom: wife, wantTo: husband, wantAmount: 40_000,
		},
		{
			name:          "精算不要",
			weightHusband: 1, weightWife: 1,
			incomeHusband: 80_000, incomeWife: 80_000,
			paidHusband: 10_000, paidWife: 10_000,
			wantNil: true,
		},
		{
			name:          "支出なしでも収入差は精算される",
			weightHusband: 1, weightWife: 1,
			incomeHusband: 100_000, incomeWife: 60_000,
			paidHusband: 0, paidWife: 0,
			wantFrom: husband, wantTo: wife, wantAmount: 20_000,
		},
		{
			// net夫=100001, net妻=100000 → t=0.5 → 四捨五入で1円
			name:          "端数0.5円は切り上げ",
			weightHusband: 1, weightWife: 1,
			incomeHusband: 100_001, incomeWife: 100_000,
			paidHusband: 0, paidWife: 0,
			wantFrom: husband, wantTo: wife, wantAmount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var expenses []domain.Expense
			if tt.paidHusband > 0 {
				expenses = append(expenses, testExpense(t, "e-husband", husband, tt.paidHusband))
			}
			if tt.paidWife > 0 {
				expenses = append(expenses, testExpense(t, "e-wife", wife, tt.paidWife))
			}
			got, err := domain.CalculateSettlement(domain.SettlementInput{
				Month:  testMonth(t),
				Couple: testCouple(t),
				Salaries: []domain.Salary{
					testSalary(t, husband, tt.incomeHusband),
					testSalary(t, wife, tt.incomeWife),
				},
				Expenses: expenses,
				Weight:   testWeight(t, tt.weightHusband, tt.weightWife),
			})
			if err != nil {
				t.Fatalf("CalculateSettlement: %v", err)
			}

			if tt.wantNil {
				if got.Transfer != nil {
					t.Fatalf("Transfer = %+v, want nil", got.Transfer)
				}
			} else {
				if got.Transfer == nil {
					t.Fatal("Transfer = nil, want transfer")
				}
				if got.Transfer.From != tt.wantFrom || got.Transfer.To != tt.wantTo || got.Transfer.Amount != tt.wantAmount {
					t.Errorf("Transfer = %s→%s %s, want %s→%s %s",
						got.Transfer.From, got.Transfer.To, got.Transfer.Amount,
						tt.wantFrom, tt.wantTo, tt.wantAmount)
				}
			}

			// 精算後の可処分所得の合計は精算前の純額合計と一致する（精算は移転のみ）。
			wantTotal := tt.incomeHusband + tt.incomeWife - tt.paidHusband - tt.paidWife
			gotTotal := got.Members[0].Disposable + got.Members[1].Disposable
			if gotTotal != wantTotal {
				t.Errorf("可処分所得の合計 = %s, want %s", gotTotal, wantTotal)
			}
			if got.TotalExpense != tt.paidHusband+tt.paidWife {
				t.Errorf("TotalExpense = %s, want %s", got.TotalExpense, tt.paidHusband+tt.paidWife)
			}
		})
	}
}

func TestCalculateSettlementDisposableRatio(t *testing.T) {
	// ユーザー例の検証: 精算後の可処分所得がともに5.5万円になる。
	got, err := domain.CalculateSettlement(domain.SettlementInput{
		Month:  testMonth(t),
		Couple: testCouple(t),
		Salaries: []domain.Salary{
			testSalary(t, husband, 100_000),
			testSalary(t, wife, 50_000),
		},
		Expenses: []domain.Expense{
			testExpense(t, "e1", husband, 20_000),
			testExpense(t, "e2", wife, 20_000),
		},
		Weight: testWeight(t, 1, 1),
	})
	if err != nil {
		t.Fatalf("CalculateSettlement: %v", err)
	}
	for _, m := range got.Members {
		if m.Disposable != 55_000 {
			t.Errorf("%s の精算後可処分所得 = %s, want 55000円", m.Member.ID, m.Disposable)
		}
	}
}

func testDirectTransfer(t *testing.T, id string, from, to domain.MemberID, amount domain.Money, month domain.YearMonth) domain.DirectTransfer {
	t.Helper()
	dt, err := domain.NewDirectTransfer(id, from, to, amount, "立替精算", month)
	if err != nil {
		t.Fatalf("NewDirectTransfer: %v", err)
	}
	return dt
}

func TestCalculateSettlementWithDirectTransfers(t *testing.T) {
	// 比重1:1、夫(収入10万・支出2万)、妻(収入5万・支出2万) → 精算のみなら夫→妻2.5万。
	// これに夫→妻5000（継続）と妻→夫2000（当月単発）の立替精算を加える。
	// 精算分: 夫→妻 25000 / 立替精算純額: 夫→妻 (5000-2000)=3000 / 合計: 夫→妻 28000。
	month := testMonth(t)
	got, err := domain.CalculateSettlement(domain.SettlementInput{
		Month:  month,
		Couple: testCouple(t),
		Salaries: []domain.Salary{
			testSalary(t, husband, 100_000),
			testSalary(t, wife, 50_000),
		},
		Expenses: []domain.Expense{
			testExpense(t, "e1", husband, 20_000),
			testExpense(t, "e2", wife, 20_000),
		},
		DirectTransfers: []domain.DirectTransfer{
			testDirectTransfer(t, "dtr_a", husband, wife, 5_000, domain.YearMonth{}),
			testDirectTransfer(t, month.String()+"_b", wife, husband, 2_000, month),
		},
		Weight: testWeight(t, 1, 1),
	})
	if err != nil {
		t.Fatalf("CalculateSettlement: %v", err)
	}

	if got.SettlementTransfer == nil || got.SettlementTransfer.From != husband || got.SettlementTransfer.Amount != 25_000 {
		t.Errorf("SettlementTransfer = %+v, want 夫→妻 25000", got.SettlementTransfer)
	}
	if got.DirectTransfer == nil || got.DirectTransfer.From != husband || got.DirectTransfer.Amount != 3_000 {
		t.Errorf("DirectTransfer = %+v, want 夫→妻 3000", got.DirectTransfer)
	}
	if got.Transfer == nil || got.Transfer.From != husband || got.Transfer.Amount != 28_000 {
		t.Errorf("Transfer = %+v, want 夫→妻 28000", got.Transfer)
	}
	if got.TotalDirectTransfer != 7_000 {
		t.Errorf("TotalDirectTransfer = %s, want 7000", got.TotalDirectTransfer)
	}
	// 可処分所得は共有支出の比重按分結果のまま（立替精算は含めない）→ ともに55000。
	for _, m := range got.Members {
		if m.Disposable != 55_000 {
			t.Errorf("%s の可処分所得 = %s, want 55000（立替精算は含めない）", m.Member.ID, m.Disposable)
		}
	}
}

func TestCalculateSettlementDirectTransfersCancelOut(t *testing.T) {
	// 立替精算が相殺され純額0のとき、精算分のみが振込になる。
	month := testMonth(t)
	got, err := domain.CalculateSettlement(domain.SettlementInput{
		Month:  month,
		Couple: testCouple(t),
		Salaries: []domain.Salary{
			testSalary(t, husband, 80_000),
			testSalary(t, wife, 80_000),
		},
		DirectTransfers: []domain.DirectTransfer{
			testDirectTransfer(t, "dtr_a", husband, wife, 5_000, domain.YearMonth{}),
			testDirectTransfer(t, "dtr_b", wife, husband, 5_000, domain.YearMonth{}),
		},
		Weight: testWeight(t, 1, 1),
	})
	if err != nil {
		t.Fatalf("CalculateSettlement: %v", err)
	}
	if got.SettlementTransfer != nil {
		t.Errorf("SettlementTransfer = %+v, want nil", got.SettlementTransfer)
	}
	if got.DirectTransfer != nil {
		t.Errorf("DirectTransfer = %+v, want nil（純額0）", got.DirectTransfer)
	}
	if got.Transfer != nil {
		t.Errorf("Transfer = %+v, want nil", got.Transfer)
	}
	if got.TotalDirectTransfer != 10_000 {
		t.Errorf("TotalDirectTransfer = %s, want 10000", got.TotalDirectTransfer)
	}
}

func TestCalculateSettlementIncomeNotReady(t *testing.T) {
	_, err := domain.CalculateSettlement(domain.SettlementInput{
		Month:  testMonth(t),
		Couple: testCouple(t),
		Salaries: []domain.Salary{
			testSalary(t, husband, 100_000), // 妻の収入が未入力
		},
		Weight: testWeight(t, 1, 1),
	})
	if !errors.Is(err, domain.ErrIncomeNotReady) {
		t.Fatalf("err = %v, want ErrIncomeNotReady", err)
	}
}

func TestCalculateSettlementAddsIncomes(t *testing.T) {
	// 給与は夫8万・妻8万で精算不要のはずだが、夫に追加収入2万を足すと
	// 収入が10万対8万となり、比重1:1では夫→妻1万の精算が生じる。
	month := testMonth(t)
	rid := string(domain.NewRecurringIncomeID("side"))
	inc, err := domain.NewIncome(rid, husband, 20_000, "副業", domain.YearMonth{})
	if err != nil {
		t.Fatalf("NewIncome: %v", err)
	}
	got, err := domain.CalculateSettlement(domain.SettlementInput{
		Month:  month,
		Couple: testCouple(t),
		Salaries: []domain.Salary{
			testSalary(t, husband, 80_000),
			testSalary(t, wife, 80_000),
		},
		Incomes: []domain.Income{inc},
		Weight:  testWeight(t, 1, 1),
	})
	if err != nil {
		t.Fatalf("CalculateSettlement: %v", err)
	}
	if got.Members[0].Income != 100_000 {
		t.Errorf("夫の収入 = %s, want 100000（給与8万＋追加収入2万）", got.Members[0].Income)
	}
	if got.Transfer == nil || got.Transfer.From != husband || got.Transfer.To != wife || got.Transfer.Amount != 10_000 {
		t.Errorf("Transfer = %+v, want 夫→妻 10000", got.Transfer)
	}
}

func TestCalculateSettlementRejectsOtherMonth(t *testing.T) {
	otherDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	e, err := domain.NewExpense("e-other", husband, 1000, "先月の支出", otherDate, otherDate)
	if err != nil {
		t.Fatalf("NewExpense: %v", err)
	}
	_, err = domain.CalculateSettlement(domain.SettlementInput{
		Month:  testMonth(t),
		Couple: testCouple(t),
		Salaries: []domain.Salary{
			testSalary(t, husband, 100_000),
			testSalary(t, wife, 50_000),
		},
		Expenses: []domain.Expense{e},
		Weight:   testWeight(t, 1, 1),
	})
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}
