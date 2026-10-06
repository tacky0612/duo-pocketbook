// デモモードの初期シードデータを生成する。
//
// 実行時の「今月」を基準に、今月（未精算）と過去11か月分（精算済み）のデータを作るため、
// いつデモを触っても精算・履歴（共有費の推移グラフ）が自然に見える。各オブジェクトのフィールド名は実 API（Go ハンドラの
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
  { id: "hanako", name: "アカウントB", color: "#ea580c" },
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
  // 精算済みにする過去の月（先月から11か月前まで）。履歴画面の初期表示（12か月）に収まる数にする。
  const pastMonths = Array.from({ length: PAST_MONTHS }, (_, i) => shiftMonth(now, -(i + 1)));

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
    reservationId: "",
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
    reservationId: "",
  });
  const incOnce = (month: string, memberId: MemberId, amountYen: number, description: string): Income => ({
    id: `${month}_${nextHex()}`,
    memberId,
    amountYen,
    description,
    recurring: false,
    month,
    reservationId: "",
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
      // 過去の月（精算済み）
      ...pastMonths.flatMap((d, i) => pastExpenses(i + 1).map(([day, paidBy, amount, desc]) => exp(ymOf(d), day, paidBy, amount, desc))),
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
      // 過去の月は少しずつ揺らす（決定的な値にして、リセットしても同じ履歴になるようにする）
      ...pastMonths.flatMap((d, i) => [
        sal(ymOf(d), "taro", 315000 + ((i * 3) % 4) * 2500),
        sal(ymOf(d), "hanako", 260000 + ((i * 5) % 4) * 6000),
      ]),
    ],
    incomes: [
      // 毎月継続: アカウントA の副業収入
      incRec("taro", 20000, "副業"),
      // 今月だけ: アカウントB の臨時収入
      incOnce(m0, "hanako", 15000, "臨時収入"),
    ],
    // 予約（金額未確定の予定）。今月は電気代・賞与が未入力の状態で始まる。
    reservations: [
      { id: `rsv_${nextHex()}`, kind: "expense", memberId: "taro", description: "電気代", recurring: true, month: "", startMonth: m0 },
      { id: `rsv_${nextHex()}`, kind: "income", memberId: "hanako", description: "賞与", recurring: false, month: m0, startMonth: "" },
    ],
    // スナップショットは下で過去の月を精算済みとして埋める
    snapshots: {},
    // 締め日は暦月どおり（1）を初期値にする
    closingDay: 1,
  };

  // 過去の月は精算完了済みとして、その時点のスナップショットを保存する。今月は未精算のまま。
  db.snapshots = Object.fromEntries(pastMonths.map((d) => [ymOf(d), snapshotFor(db, ymOf(d))]));
  return db;
}

type PastItem = [day: number, paidBy: MemberId, amountYen: number, description: string];

// 過去の月ごとの支出パターン（添字0が先月）。推移に起伏が出るよう、
// 日々の買い物の量（scale）と主に買い物をした人（lead）を月ごとに変え、
// 大きな出費（events）のある月・ない月を混ぜて、支出の偏りが A・B どちらにも出るようにする。
const PAST_PLANS: { scale: number; lead: MemberId; events: PastItem[] }[] = [
  { scale: 1.1, lead: "hanako", events: [[13, "hanako", 62000, "旅行"]] },
  { scale: 0.6, lead: "taro", events: [] },
  { scale: 1.0, lead: "taro", events: [[7, "taro", 45000, "家具"], [22, "taro", 9800, "外食（記念日）"]] },
  { scale: 1.3, lead: "hanako", events: [[16, "hanako", 38000, "家電の買い替え"], [25, "hanako", 8500, "医療費"]] },
  { scale: 0.7, lead: "hanako", events: [] },
  { scale: 1.0, lead: "taro", events: [[10, "taro", 26000, "車の点検"]] },
  { scale: 1.2, lead: "hanako", events: [[19, "hanako", 30000, "冠婚葬祭"]] },
  { scale: 1.4, lead: "taro", events: [[3, "taro", 68000, "旅行"]] },
  { scale: 0.75, lead: "taro", events: [] },
  { scale: 1.1, lead: "hanako", events: [[12, "hanako", 24000, "家具"], [27, "hanako", 9000, "日用品のまとめ買い"]] },
  { scale: 0.9, lead: "taro", events: [[8, "taro", 18000, "家電"]] },
];

// 精算済みにする過去の月数。
const PAST_MONTHS = PAST_PLANS.length;

// pastExpenses は過去の月（ago か月前）の通常の共有支出を返す。
// 乱数は使わず PAST_PLANS から決定的に金額を決める（リセットしても同じ履歴になる）。
function pastExpenses(ago: number): PastItem[] {
  const { scale, lead, events } = PAST_PLANS[ago - 1];
  const other: MemberId = lead === "taro" ? "hanako" : "taro";
  const amt = (base: number) => Math.round((base * scale) / 100) * 100;
  return [
    // 主に買い物をした人（lead）が日々の支出の大半を払う
    [5, lead, amt(14000), "スーパー"],
    [18, lead, amt(11000), "スーパー"],
    [11, lead, amt(4500), "日用品"],
    [24, other, amt(5000), "スーパー"],
    [15, other, amt(6000), "外食"],
    ...events,
  ];
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
