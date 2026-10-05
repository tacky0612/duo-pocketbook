// アプリ全体で共有するドメイン／API の型定義。
// フィールド名はバックエンド（Go ハンドラの json タグ）に一致させている。

// --- 値の別名 ---
export type MemberId = string;
export type YearMonth = string; // "YYYY-MM"
export type IsoDate = string; // "YYYY-MM-DD"

// --- メンバー ---
/** ログインレスポンス等で使う最小のメンバー情報。 */
export interface Member {
  id: MemberId;
  name: string;
}

/** /members が返す表示用メンバー（カラー付き）。 */
export interface MemberView extends Member {
  color: string;
}

// --- 支出・固定費・収入 ---
export interface Expense {
  id: string;
  paidBy: MemberId;
  amountYen: number;
  description: string;
  date: IsoDate;
  month: YearMonth;
  createdAt: string;
  /** 予約の入力で登録された支出ならその予約ID。通常の支出は空文字。 */
  reservationId: string;
}

export interface RecurringExpense {
  id: string;
  paidBy: MemberId;
  amountYen: number;
  description: string;
}

/** 立替精算（共有支出とは別の A→B 送金）。recurring=true は毎月継続、false は month の単発。 */
export interface DirectTransfer {
  id: string;
  from: MemberId;
  to: MemberId;
  amountYen: number;
  description: string;
  recurring: boolean;
  month: YearMonth; // 継続は空文字
}

/** 給与（毎月発生する基本の収入）。メンバーごと・月ごとに1件。精算の可否判定に使う。 */
export interface Salary {
  memberId: MemberId;
  amountYen: number;
}

/** 給与とは別の追加収入（副業など）。recurring=true は毎月継続、false は month の単発。日付は持たない。 */
export interface Income {
  id: string;
  memberId: MemberId;
  amountYen: number;
  description: string;
  recurring: boolean;
  month: YearMonth; // 継続は空文字
  /** 予約の入力で登録された収入ならその予約ID。通常の収入は空文字。 */
  reservationId: string;
}

// --- 予約 ---
/** 予約の種別。expense=支出の予約、income=収入の予約。 */
export type ReservationKind = "expense" | "income";

/** 精算月ごとの予約の入力状況。pending=未入力 / fulfilled=入力済み / skipped=今月はなし。 */
export type ReservationStatus = "pending" | "fulfilled" | "skipped";

/**
 * 金額が確定していない支出・収入の予約と、指定精算月の入力状況。
 * 予約自体は精算に影響せず、金額を入力すると支出／収入として登録される。
 */
export interface Reservation {
  /** 頻度に依存しない固定の予約ID（rsv_<hex>）。 */
  id: string;
  kind: ReservationKind;
  /** 支出なら支払う人、収入なら収入を得る人。 */
  memberId: MemberId;
  description: string;
  /** 見込み額。0 は未定。 */
  estimatedAmountYen: number;
  recurring: boolean;
  month: YearMonth; // 毎月は空文字
  /** 毎月の予約の開始月。これより前の月には現れない。空文字は制限なし（単発は常に空文字）。 */
  startMonth: YearMonth | "";
  status: ReservationStatus;
  /** 入力済みのとき、対象月に予約へ紐づく支出ID／収入IDのすべて（通常は1件）。 */
  fulfilledIds: string[];
  /** 入力済みのとき、対象月に紐づく支出／収入の合計額。 */
  fulfilledAmountYen: number;
}

// --- 精算 ---
export interface MemberSettlement {
  id: MemberId;
  name: string;
  weight: number;
  incomeYen: number;
  paidExpenseYen: number;
  disposableYen: number;
}

export interface Transfer {
  from: MemberId;
  to: MemberId;
  amountYen: number;
}

export interface Settlement {
  month: YearMonth;
  totalExpenseYen: number;
  members: MemberSettlement[];
  /** 実際の振込（精算分＋立替精算分の合算）。0円なら null。 */
  transfer: Transfer | null;
  /** 比重按分による精算分のみの振込。0円なら null。 */
  settlementTransfer: Transfer | null;
  /** 立替精算の純額のみの振込。0円なら null。 */
  directTransfer: Transfer | null;
  /** 当月に適用された立替精算の総額（方向を問わない絶対額の合計）。 */
  totalDirectTransferYen: number;
  settled: boolean;
}

/** 精算スナップショットに含まれる共有支出1件の明細。 */
export interface SnapshotExpense {
  paidBy: MemberId;
  amountYen: number;
  description: string;
  date: IsoDate | ""; // 固定費など日付を持たない項目は空文字
  recurring: boolean;
}

/** 精算スナップショットに含まれる立替精算1件の明細。 */
export interface SnapshotDirectTransfer {
  from: MemberId;
  to: MemberId;
  amountYen: number;
  description: string;
  recurring: boolean;
}

/**
 * 精算完了時点の内容を凍結したスナップショット（精算履歴の1件）。
 * 元データ（給与・支出・固定費・比重など）を後から変更しても内容は変わらない。
 */
export interface SettlementHistoryEntry {
  month: YearMonth;
  settledAt: string; // RFC3339
  totalExpenseYen: number;
  members: MemberSettlement[];
  /** 実際の振込（精算分＋立替精算分の合算）。0円なら null。 */
  transfer: Transfer | null;
  /** 比重按分による精算分のみの振込。0円なら null。 */
  settlementTransfer: Transfer | null;
  /** 立替精算の純額のみの振込。0円なら null。 */
  directTransfer: Transfer | null;
  /** 当月に適用された立替精算の総額（方向を問わない絶対額の合計）。 */
  totalDirectTransferYen: number;
  expenses: SnapshotExpense[];
  directTransfers: SnapshotDirectTransfer[];
}

export type Weights = Record<MemberId, number>;

// --- API レスポンスのラッパー ---
export interface LoginResponse {
  token: string;
  member: Member;
  expiresAt: string;
}

/** GET /account: 認証中アカウントの不変ID・可変ログインID・表示名。 */
export interface AccountResponse {
  accountId: string;
  loginId: string;
  name: string;
}
export interface MembersResponse {
  members: MemberView[];
}
export interface ExpensesResponse {
  month: YearMonth;
  expenses: Expense[];
}
export interface SalariesResponse {
  month: YearMonth;
  salaries: Salary[];
}
export interface SalaryResponse {
  month: YearMonth;
  salary: Salary;
}
export interface IncomesResponse {
  month: YearMonth;
  incomes: Income[];
}
export interface IncomeResponse {
  month: YearMonth;
  income: Income;
}
export interface RecurringExpensesResponse {
  recurringExpenses: RecurringExpense[];
}
export interface ReservationsResponse {
  month: YearMonth;
  reservations: Reservation[];
  /** 未入力（status=pending）の予約の件数。 */
  pendingCount: number;
  /** 対象月が精算確定済みか（確定済みの月は金額入力・今月はなしができない）。 */
  settled: boolean;
}
export interface DirectTransfersResponse {
  month: YearMonth;
  directTransfers: DirectTransfer[];
}
export interface WeightsResponse {
  weights: Weights;
}
export interface ClosingDayResponse {
  closingDay: number;
}
export interface SettlementStatusResponse {
  month: YearMonth;
  settled: boolean;
}
export interface HistoryResponse {
  entries: SettlementHistoryEntry[];
}

// --- エラー ---
export type ErrorCode =
  | "VALIDATION_ERROR"
  | "UNAUTHORIZED"
  | "NOT_FOUND"
  | "INCOME_NOT_READY"
  | "MONTH_SETTLED"
  | "INTERNAL";

/** エラーレスポンスの JSON ボディ。 */
export interface ApiErrorBody {
  error?: { code?: string; message?: string };
}

// --- UI 横断 ---
export type ScreenName =
  | "settlement"
  | "income"
  | "expense"
  | "directTransfer"
  | "history"
  | "settings";

/** 支出画面のタブ。 */
export type ExpenseTab = "variable" | "fixed" | "reservation";

/** 画面遷移時の追加指定（遷移先画面の初期タブなど）。 */
export interface NavigateOptions {
  expenseTab?: ExpenseTab;
  /** 支出画面の予約タブで、金額入力フォームを開いた状態にする予約のID。 */
  fulfillReservationId?: string;
}

export type ToastKind = "success" | "error";

export interface ToastMessage {
  message: string;
  kind: ToastKind;
  at: number;
}

export type Notify = (message: string, kind?: ToastKind) => void;

/** 各画面へ渡す共通 props。 */
export interface ScreenProps {
  month: YearMonth;
  members: MemberView[];
  me: Member;
  notify: Notify;
  onError: (err: unknown) => void;
  onNavigate: (screen: ScreenName, options?: NavigateOptions) => void;
  onMonthChange: (month: YearMonth) => void;
  /** 締め日（精算期間の起算日。1=暦月どおり）。App がログイン時に取得し、設定の保存で更新する。 */
  closingDay: number;
}

// --- テーマ ---
export type ThemeMode = "light" | "dark" | "system";

export interface Theme {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
}

// --- デモモード ---
/** デモの月次給与（内部保持用。month を持つ点が API の Salary と異なる）。 */
export interface DemoSalary {
  month: YearMonth;
  memberId: MemberId;
  amountYen: number;
}

/** デモの予約（内部保持用。月ごとの入力状況は持たない）。 */
export type DemoReservation = Omit<Reservation, "status" | "fulfilledIds" | "fulfilledAmountYen"> & {
  /** 「今月はなし」にした精算月（毎月の予約のみ）。 */
  skippedMonths?: YearMonth[];
};

/** デモのインメモリDB（localStorage に保存される）。 */
export interface DemoDb {
  members: MemberView[];
  weights: Weights;
  expenses: Expense[];
  recurring: RecurringExpense[];
  directTransfers: DirectTransfer[];
  salaries: DemoSalary[];
  incomes: Income[];
  /** 予約（単発・毎月）。入力状況は都度 expenses / incomes の reservationId と skippedMonths から判定する。 */
  reservations: DemoReservation[];
  /** 精算完了時点のスナップショット（月ごと）。存在＝精算済み。 */
  snapshots: Record<YearMonth, SettlementHistoryEntry>;
  /** 締め日（精算期間の起算日。1=暦月どおり）。1〜31。 */
  closingDay: number;
}
