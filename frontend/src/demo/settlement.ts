// 精算計算のデモ実装。
//
// バックエンドの internal/domain/settlement.go（CalculateSettlement）と
// internal/application/settlement_usecase.go（GetSettlement: 固定費の実体化・
// 既定比重1:1・収入未入力の扱い）を JS へ移植したもの。計算ロジックは同一に保つ。

import { ApiError } from "../lib/apiClient";
import { settlementMonthOf } from "../lib/month";
import type { DemoIncome, DirectTransfer, Expense, Member, RecurringExpense, Settlement, Transfer, Weights } from "../types";

export { settlementMonthOf };

export interface SettlementInput {
  month: string;
  members: Member[];
  weights: Weights;
  incomes: DemoIncome[];
  expenses: Expense[];
  recurring: RecurringExpense[];
  directTransfers: DirectTransfer[];
  closingDay: number;
}

// settled を除いた精算結果（settled は呼び出し側でストアから付与する）。
export type ComputedSettlement = Omit<Settlement, "settled">;

// roundDiv は num/den を四捨五入（絶対値で half away from zero）した整数を返す。den は正であること。
function roundDiv(num: number, den: number): number {
  const neg = num < 0;
  const n = neg ? -num : num;
  const q = Math.floor((n + Math.floor(den / 2)) / den);
  return neg ? -q : q;
}

// computeSettlement は対象月の精算結果（DTO 相当）を返す。
// 両メンバーの収入が未入力の場合は INCOME_NOT_READY を投げる。
// transferBetween は a→b の符号付き金額から Transfer を組み立てる（0 は null）。
function transferBetween(aId: string, bId: string, signed: number): Transfer | null {
  if (signed > 0) return { from: aId, to: bId, amountYen: signed };
  if (signed < 0) return { from: bId, to: aId, amountYen: -signed };
  return null;
}

export function computeSettlement({
  month,
  members,
  weights,
  incomes,
  expenses,
  recurring,
  directTransfers,
  closingDay,
}: SettlementInput): ComputedSettlement {
  const [a, b] = members;
  const wA = weights[a.id] ?? 1;
  const wB = weights[b.id] ?? 1;

  const incomeOf = (id: string): number | null => {
    const found = incomes.find((i) => i.month === month && i.memberId === id);
    return found ? found.amountYen : null;
  };
  const incomeA = incomeOf(a.id);
  const incomeB = incomeOf(b.id);
  if (incomeA == null || incomeB == null) {
    throw new ApiError(`収入が未入力です (対象月: ${month})`, "INCOME_NOT_READY", 409);
  }

  // 対象月の支出に固定費を実体化して加算する（全月共通で毎月発生する共有支出）。
  const paid: Record<string, number> = { [a.id]: 0, [b.id]: 0 };
  let total = 0;
  for (const e of expenses) {
    if (settlementMonthOf(e.date, closingDay) !== month) continue;
    paid[e.paidBy] = (paid[e.paidBy] || 0) + e.amountYen;
    total += e.amountYen;
  }
  for (const r of recurring) {
    paid[r.paidBy] = (paid[r.paidBy] || 0) + r.amountYen;
    total += r.amountYen;
  }

  const netA = incomeA - (paid[a.id] || 0);
  const netB = incomeB - (paid[b.id] || 0);

  // t > 0 なら a → b、t < 0 なら b → a への精算振込（比重按分）。
  const t = roundDiv(wB * netA - wA * netB, wA + wB);

  // 立替精算は比重按分に含めず、a→b の純額 d として集計する。
  let d = 0;
  let totalDirect = 0;
  for (const dt of directTransfers) {
    if (dt.from === a.id) d += dt.amountYen;
    else d -= dt.amountYen;
    totalDirect += dt.amountYen;
  }

  return {
    month,
    totalExpenseYen: total,
    // 可処分所得は共有支出の比重按分結果のみを反映する（立替精算は別枠）。
    members: [
      { id: a.id, name: a.name, weight: wA, incomeYen: incomeA, paidExpenseYen: paid[a.id] || 0, disposableYen: netA - t },
      { id: b.id, name: b.name, weight: wB, incomeYen: incomeB, paidExpenseYen: paid[b.id] || 0, disposableYen: netB + t },
    ],
    transfer: transferBetween(a.id, b.id, t + d),
    settlementTransfer: transferBetween(a.id, b.id, t),
    directTransfer: transferBetween(a.id, b.id, d),
    totalDirectTransferYen: totalDirect,
  };
}
