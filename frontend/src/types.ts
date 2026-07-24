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

export interface SettlementHistoryEntry {
  month: YearMonth;
  settled: boolean;
  totalExpenseYen: number;
  transfer: Transfer | null;
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
  | "recurring"
  | "directTransfer"
  | "history"
  | "settings";

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
  onNavigate: (screen: ScreenName) => void;
  onMonthChange: (month: YearMonth) => void;
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

/** デモのインメモリDB（localStorage に保存される）。 */
export interface DemoDb {
  members: MemberView[];
  weights: Weights;
  expenses: Expense[];
  recurring: RecurringExpense[];
  directTransfers: DirectTransfer[];
  salaries: DemoSalary[];
  incomes: Income[];
  settled: Record<YearMonth, boolean>;
  /** 締め日（精算期間の起算日。1=暦月どおり）。1〜31。 */
  closingDay: number;
}
