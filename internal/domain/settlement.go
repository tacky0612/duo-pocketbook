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
	// Transfer は実際に必要な振込（精算分＋立替精算分の合算）。0円の場合は nil。
	Transfer *Transfer
	// SettlementTransfer は比重按分による精算分のみの振込。0円の場合は nil。
	SettlementTransfer *Transfer
	// DirectTransfer は立替精算の純額のみの振込。0円の場合は nil。
	DirectTransfer *Transfer
	// TotalDirectTransfer は立替精算の総額（方向を問わない絶対額の合計）。
	TotalDirectTransfer Money
}

// SettlementInput は精算計算への入力。
type SettlementInput struct {
	Month   YearMonth
	Couple  Couple
	Incomes []MonthlyIncome // 両メンバー分が揃っている必要がある
	// Expenses は対象精算月に計上する共有支出。締め日設定により暦月をまたぐ場合がある。
	Expenses []Expense
	// DirectTransfers は対象月に適用する立替精算（毎月継続分＋当月単発分）。
	// 比重按分に含めず、振込額へ純額として加算する。
	DirectTransfers []DirectTransfer
	Weight          Weight
	// ClosingDay は締め日。支出が対象月に属するかの検証に使う。
	// ゼロ値は暦月どおり（DefaultClosingDay と同じ）として扱う。
	ClosingDay ClosingDay
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
		if in.ClosingDay.SettlementMonth(e.Date) != in.Month {
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

	// t > 0 なら a → b、t < 0 なら b → a への精算振込（比重按分）。
	t := roundDiv(weightB*int64(netA)-weightA*int64(netB), weightA+weightB)

	// 立替精算は比重按分に含めず、a→b の純額 d として集計する（t とは独立）。
	var d, totalDirect int64
	for _, dt := range in.DirectTransfers {
		if !in.Couple.Contains(dt.From) || !in.Couple.Contains(dt.To) || dt.From == dt.To {
			return nil, fmt.Errorf("%w: 不明なメンバーの立替精算が含まれています: %s", ErrValidation, dt.ID)
		}
		if !dt.Month.IsZero() && dt.Month != in.Month {
			return nil, fmt.Errorf("%w: 対象月以外の立替精算が含まれています: %s", ErrValidation, dt.ID)
		}
		if dt.From == a.ID {
			d += int64(dt.Amount)
		} else {
			d -= int64(dt.Amount)
		}
		totalDirect += int64(dt.Amount)
	}

	// 可処分所得は共有支出の比重按分結果のみを反映する（立替精算は別枠のため含めない）。
	settlement := &Settlement{
		Month:               in.Month,
		TotalExpense:        total,
		TotalDirectTransfer: Money(totalDirect),
		Members: [2]MemberSettlement{
			{Member: a, Weight: weightA, Income: incomeA, PaidExpense: paid[a.ID], Disposable: netA.Sub(Money(t))},
			{Member: b, Weight: weightB, Income: incomeB, PaidExpense: paid[b.ID], Disposable: netB.Add(Money(t))},
		},
		SettlementTransfer: transferBetween(a.ID, b.ID, t),
		DirectTransfer:     transferBetween(a.ID, b.ID, d),
		Transfer:           transferBetween(a.ID, b.ID, t+d),
	}
	return settlement, nil
}

// transferBetween は a→b の符号付き金額から Transfer を組み立てる。
// signed > 0 なら a→b、signed < 0 なら b→a、0 なら nil を返す。
func transferBetween(a, b MemberID, signed int64) *Transfer {
	switch {
	case signed > 0:
		return &Transfer{From: a, To: b, Amount: Money(signed)}
	case signed < 0:
		return &Transfer{From: b, To: a, Amount: Money(-signed)}
	default:
		return nil
	}
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
