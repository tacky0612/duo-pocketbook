package domain

import "fmt"

// MemberSettlement は精算結果におけるメンバーごとの内訳。
type MemberSettlement struct {
	Member      Member
	Weight      int64
	Income      Money // 対象月の収入
	PaidExpense Money // 対象月に立て替えた共有支出の合計
	Disposable  Money // 精算後の可処分所得 (= 収入 - 立替支出 ± 精算額)
}

// Transfer は精算のための振込を表す。
type Transfer struct {
	From   MemberID
	To     MemberID
	Amount Money // 正の金額
}

// Settlement はある月の精算結果。
type Settlement struct {
	Month        YearMonth
	TotalExpense Money
	Members      [2]MemberSettlement
	// Transfer は精算に必要な振込。精算額が0円の場合は nil。
	Transfer *Transfer
}

// SettlementInput は精算計算への入力。
type SettlementInput struct {
	Month    YearMonth
	Couple   Couple
	Incomes  []MonthlyIncome // 両メンバー分が揃っている必要がある
	Expenses []Expense       // 対象月の共有支出
	Weight   Weight
}

// CalculateSettlement は精算額を計算するドメインサービス。
//
// 各メンバーの純額 net = 収入 - 立替済み共有支出 とし、比重 wA:wB に対して
// 精算後の可処分所得が dispA/wA == dispB/wB となるよう、
// AからBへの振込額 t = (wB*netA - wA*netB) / (wA + wB) を求める
// （端数は四捨五入、t が負の場合はBからAへの振込）。
//
// 例: 比重1:1、夫(収入10万・立替2万)、妻(収入5万・立替2万)
// → 夫から妻へ2.5万円振り込むと双方の可処分所得が5.5万円で等しくなる。
func CalculateSettlement(in SettlementInput) (*Settlement, error) {
	if in.Month.IsZero() {
		return nil, fmt.Errorf("%w: 対象年月は必須です", ErrValidation)
	}
	members := in.Couple.Members()
	a, b := members[0], members[1]

	weightA, okA := in.Weight.Of(a.ID)
	weightB, okB := in.Weight.Of(b.ID)
	if !okA || !okB {
		return nil, fmt.Errorf("%w: 比重に両メンバーの設定が必要です", ErrValidation)
	}

	incomes := map[MemberID]Money{}
	for _, inc := range in.Incomes {
		if inc.Month != in.Month {
			return nil, fmt.Errorf("%w: 対象月以外の収入が含まれています: %s", ErrValidation, inc.Month)
		}
		if !in.Couple.Contains(inc.MemberID) {
			return nil, fmt.Errorf("%w: 不明なメンバーの収入が含まれています: %s", ErrValidation, inc.MemberID)
		}
		incomes[inc.MemberID] = inc.Amount
	}
	incomeA, okA := incomes[a.ID]
	incomeB, okB := incomes[b.ID]
	if !okA || !okB {
		return nil, fmt.Errorf("%w (対象月: %s)", ErrIncomeNotReady, in.Month)
	}

	paid := map[MemberID]Money{}
	var total Money
	for _, e := range in.Expenses {
		if e.Month() != in.Month {
			return nil, fmt.Errorf("%w: 対象月以外の支出が含まれています: %s", ErrValidation, e.ID)
		}
		if !in.Couple.Contains(e.PaidBy) {
			return nil, fmt.Errorf("%w: 不明なメンバーの支出が含まれています: %s", ErrValidation, e.PaidBy)
		}
		paid[e.PaidBy] = paid[e.PaidBy].Add(e.Amount)
		total = total.Add(e.Amount)
	}

	netA := incomeA.Sub(paid[a.ID])
	netB := incomeB.Sub(paid[b.ID])

	// t > 0 なら a → b、t < 0 なら b → a への振込。
	t := roundDiv(weightB*int64(netA)-weightA*int64(netB), weightA+weightB)

	settlement := &Settlement{
		Month:        in.Month,
		TotalExpense: total,
		Members: [2]MemberSettlement{
			{Member: a, Weight: weightA, Income: incomeA, PaidExpense: paid[a.ID], Disposable: netA.Sub(Money(t))},
			{Member: b, Weight: weightB, Income: incomeB, PaidExpense: paid[b.ID], Disposable: netB.Add(Money(t))},
		},
	}
	switch {
	case t > 0:
		settlement.Transfer = &Transfer{From: a.ID, To: b.ID, Amount: Money(t)}
	case t < 0:
		settlement.Transfer = &Transfer{From: b.ID, To: a.ID, Amount: Money(-t)}
	}
	return settlement, nil
}

// roundDiv は num/den を四捨五入（絶対値で half away from zero）した整数を返す。den は正であること。
func roundDiv(num, den int64) int64 {
	neg := num < 0
	if neg {
		num = -num
	}
	q := (num + den/2) / den
	if neg {
		return -q
	}
	return q
}
