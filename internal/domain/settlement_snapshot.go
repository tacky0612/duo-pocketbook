package domain

import "time"

// SettlementExpenseItem は精算スナップショットに含める共有支出1件の明細。
type SettlementExpenseItem struct {
	PaidBy      MemberID // 立て替えたメンバー
	Amount      Money    // 正の金額（円）
	Description string
	Date        string // 支出日（YYYY-MM-DD）。固定費など日付を持たない項目は空。
	Recurring   bool   // 固定費由来なら true
}

// SettlementDirectTransferItem は精算スナップショットに含める立替精算1件の明細。
type SettlementDirectTransferItem struct {
	From        MemberID
	To          MemberID
	Amount      Money // 正の金額（円）
	Description string
	Recurring   bool // 毎月継続なら true
}

// SettlementSnapshot は精算完了時点の精算内容を凍結した記録。
//
// 精算履歴は元データ（給与・収入・支出・固定費・比重・締め日）を後から変更しても
// 完了時点の内容を保ち続ける必要があるため、精算完了をトリガーにこのスナップショットを
// 保存し、履歴表示はスナップショットを参照する（再計算しない）。
type SettlementSnapshot struct {
	// Settlement は完了時点で確定した精算結果（振込額・可処分所得・合計など）。
	Settlement Settlement
	// SettledAt は精算を完了した日時。
	SettledAt time.Time
	// Expenses は当月に計上した共有支出の明細（固定費の実体化分を含む）。
	Expenses []SettlementExpenseItem
	// DirectTransfers は当月に適用した立替精算の明細。
	DirectTransfers []SettlementDirectTransferItem
}

// Month はスナップショット対象の年月を返す。
func (s SettlementSnapshot) Month() YearMonth { return s.Settlement.Month }
