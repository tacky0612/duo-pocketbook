// デモモードの初期シードデータを生成する。
//
// 実行時の「今月」を基準に直近3か月分のデータを作るため、いつデモを触っても
// 精算・履歴が自然に見える。各オブジェクトのフィールド名は実 API（Go ハンドラの
// json タグ）に厳密一致させている（amountYen / paidBy / incomeYen など）。

import type { DemoDb, DemoIncome, Expense, MemberId, MemberView, RecurringExpense } from "../types";

// デモの2アカウント。id はログインに使い、color は支出一覧のバッジ色に使う。
const MEMBERS: MemberView[] = [
  { id: "taro", name: "太郎", color: "#2563eb" },
  { id: "hanako", name: "花子", color: "#4f46e5" },
];

function ymOf(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

function shiftMonth(base: Date, delta: number): Date {
  return new Date(base.getFullYear(), base.getMonth() + delta, 1);
}

function dateStr(month: string, day: number): string {
  return `${month}-${String(day).padStart(2, "0")}`;
}

// seedData は初期状態のデモDBを生成して返す。store の初回ロード・リセット時に使う。
export function seedData(): DemoDb {
  const now = new Date();
  const m0 = ymOf(now); // 今月
  const m1 = ymOf(shiftMonth(now, -1)); // 先月
  const m2 = ymOf(shiftMonth(now, -2)); // 先々月

  let seq = 0;
  const nextHex = () => (++seq).toString(16).padStart(6, "0");

  // 通常の共有支出（対象月ごと）
  const exp = (month: string, day: number, paidBy: MemberId, amountYen: number, description: string): Expense => ({
    id: `${month}_${nextHex()}`,
    paidBy,
    amountYen,
    description,
    date: dateStr(month, day),
    month,
    createdAt: `${dateStr(month, day)}T09:00:00Z`,
  });

  // 固定費（月に依存せず全月の精算へ自動加算される）
  const rec = (id: string, paidBy: MemberId, amountYen: number, description: string): RecurringExpense => ({
    id,
    paidBy,
    amountYen,
    description,
  });

  // 月次収入
  const inc = (month: string, memberId: MemberId, amountYen: number): DemoIncome => ({ month, memberId, amountYen });

  return {
    members: MEMBERS.map((m) => ({ ...m })),
    weights: { taro: 1, hanako: 1 },
    expenses: [
      // 今月
      exp(m0, 15, "taro", 4800, "スーパー"),
      exp(m0, 12, "hanako", 2600, "日用品"),
      exp(m0, 8, "taro", 3200, "外食"),
      // 先月
      exp(m1, 20, "hanako", 5400, "スーパー"),
      exp(m1, 14, "taro", 8900, "外食"),
      exp(m1, 5, "hanako", 1800, "日用品"),
      // 先々月
      exp(m2, 18, "taro", 6200, "スーパー"),
      exp(m2, 9, "hanako", 3300, "医療費"),
    ],
    recurring: [
      rec("rent", "taro", 90000, "家賃"),
      rec("utility", "hanako", 12000, "光熱費"),
      rec("subscription", "hanako", 3000, "サブスク"),
    ],
    incomes: [
      inc(m0, "taro", 320000),
      inc(m0, "hanako", 280000),
      inc(m1, "taro", 320000),
      inc(m1, "hanako", 260000),
      inc(m2, "taro", 315000),
      inc(m2, "hanako", 280000),
    ],
    // 過去2か月は精算済み、今月は未精算にしておく
    settled: { [m1]: true, [m2]: true },
    // 締め日は暦月どおり（1）を初期値にする
    closingDay: 1,
  };
}
