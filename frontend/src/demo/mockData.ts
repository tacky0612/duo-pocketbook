// デモモードの初期シードデータを生成する。
//
// 実行時の「今月」を基準に直近3か月分のデータを作るため、いつデモを触っても
// 精算・履歴が自然に見える。各オブジェクトのフィールド名は実 API（Go ハンドラの
// json タグ）に厳密一致させている（amountYen / paidBy / incomeYen など）。

import { computeSettlement, settlementMonthOf } from "./settlement";
import type {
  DemoDb,
  DemoSalary,
  DirectTransfer,
  Expense,
  Income,
  MemberId,
  MemberView,
  RecurringExpense,
  SettlementHistoryEntry,
  SnapshotExpense,
} from "../types";

// デモの2アカウント。id はログインに使い、color は支出一覧のバッジ色に使う。
const MEMBERS: MemberView[] = [
  { id: "taro", name: "アカウントA", color: "#2563eb" },
  { id: "hanako", name: "アカウントB", color: "#4f46e5" },
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

  // 月次給与（毎月発生する基本の収入）
  const sal = (month: string, memberId: MemberId, amountYen: number): DemoSalary => ({ month, memberId, amountYen });

  // 追加収入（給与とは別。毎月継続 or 単発）
  const incRec = (memberId: MemberId, amountYen: number, description: string): Income => ({
    id: `inc_${nextHex()}`,
    memberId,
    amountYen,
    description,
    recurring: true,
    month: "",
  });
  const incOnce = (month: string, memberId: MemberId, amountYen: number, description: string): Income => ({
    id: `${month}_${nextHex()}`,
    memberId,
    amountYen,
    description,
    recurring: false,
    month,
  });

  // 立替精算（共有支出とは別の A→B 送金。比重按分せず振込額へ加算）
  const dtr = (from: MemberId, to: MemberId, amountYen: number, description: string): DirectTransfer => ({
    id: `dtr_${nextHex()}`,
    from,
    to,
    amountYen,
    description,
    recurring: true,
    month: "",
  });
  const dtOnce = (month: string, from: MemberId, to: MemberId, amountYen: number, description: string): DirectTransfer => ({
    id: `${month}_${nextHex()}`,
    from,
    to,
    amountYen,
    description,
    recurring: false,
    month,
  });

  const db: DemoDb = {
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
    directTransfers: [
      // 毎月継続: アカウントB → アカウントA へお小遣い
      dtr("hanako", "taro", 10000, "お小遣い"),
      // 今月だけ: アカウントA → アカウントB へ立替の返済
      dtOnce(m0, "taro", "hanako", 3000, "立替の返済"),
    ],
    salaries: [
      sal(m0, "taro", 320000),
      sal(m0, "hanako", 280000),
      sal(m1, "taro", 320000),
      sal(m1, "hanako", 260000),
      sal(m2, "taro", 315000),
      sal(m2, "hanako", 280000),
    ],
    incomes: [
      // 毎月継続: アカウントA の副業収入
      incRec("taro", 20000, "副業"),
      // 今月だけ: アカウントB の臨時収入
      incOnce(m0, "hanako", 15000, "臨時収入"),
    ],
    // スナップショットは下で m1・m2 を精算済みとして埋める
    snapshots: {},
    // 締め日は暦月どおり（1）を初期値にする
    closingDay: 1,
  };

  // 過去2か月（先月・先々月）は精算完了済みとして、その時点のスナップショットを保存する。
  // 今月は未精算のまま。
  db.snapshots = {
    [m1]: snapshotFor(db, m1),
    [m2]: snapshotFor(db, m2),
  };
  return db;
}

// snapshotFor はシード用に、対象月の精算内容をスナップショット（履歴エントリ）へ組み立てる。
// demoApi.buildSnapshot と同じ構造を、store 依存なしで生成する。
function snapshotFor(db: DemoDb, month: string): SettlementHistoryEntry {
  const cd = db.closingDay;
  const s = computeSettlement({
    month,
    members: db.members,
    weights: db.weights,
    salaries: db.salaries,
    incomes: db.incomes,
    expenses: db.expenses,
    recurring: db.recurring,
    directTransfers: db.directTransfers.filter((dt) => dt.recurring || dt.month === month),
    closingDay: cd,
  });
  const expenses: SnapshotExpense[] = [
    ...db.expenses
      .filter((e) => settlementMonthOf(e.date, cd) === month)
      .map((e) => ({ paidBy: e.paidBy, amountYen: e.amountYen, description: e.description, date: e.date, recurring: false })),
    ...db.recurring.map((r) => ({ paidBy: r.paidBy, amountYen: r.amountYen, description: r.description, date: "" as const, recurring: true })),
  ];
  const directTransfers = db.directTransfers
    .filter((dt) => dt.recurring || dt.month === month)
    .map((t) => ({ from: t.from, to: t.to, amountYen: t.amountYen, description: t.description, recurring: t.recurring }));
  // シードのスナップショットは対象月の末日を完了日時にしておく（当月の最終日 = 翌月0日）。
  const [y, mo] = month.split("-").map(Number);
  const settledAt = new Date(y, mo, 0).toISOString();
  return { ...s, settledAt, expenses, directTransfers };
}
