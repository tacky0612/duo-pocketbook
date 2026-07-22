// デモモードの API ルーター。
//
// apiClient.api(method, path, body) から委譲され、実バックエンド（internal/web の
// ハンドラ）と同じ形のレスポンスを in-browser のモックストアから返す。全レスポンスの
// フィールド名・エラー形状は実 API に一致させ、既存の画面コードを一切変更せず動かす。

import { ApiError, type HttpMethod } from "../lib/apiClient";
import { store } from "./store";
import { computeSettlement } from "./settlement";
import type { DemoDb, ExpensesResponse, Settlement, Weights } from "../types";

// デモが受け取り得るリクエストボディのフィールド（すべて任意）。
interface DemoBody {
  memberId?: string;
  password?: string;
  name?: string;
  color?: string;
  paidBy?: string;
  amountYen?: number;
  description?: string;
  date?: string;
  settled?: boolean;
  weights?: Weights;
}

// --- エラーヘルパー（apiClient の ApiError 形状に合わせる） ---
function notFound(message = "見つかりません"): never {
  throw new ApiError(message, "NOT_FOUND", 404);
}
function validation(message: string): never {
  throw new ApiError(message, "VALIDATION_ERROR", 400);
}
function unauthorized(message = "認証に失敗しました"): never {
  throw new ApiError(message, "UNAUTHORIZED", 401);
}

// --- 小さなユーティリティ ---
function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split("-").map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}
function monthIndex(month: string): number {
  const [y, m] = month.split("-").map(Number);
  return y * 12 + (m - 1);
}
function nowISO(): string {
  return new Date().toISOString();
}
function randHex(): string {
  return Math.random().toString(16).slice(2, 10);
}

// 支出DTO（一覧は日付降順）。
function listExpenses(db: DemoDb, month: string): ExpensesResponse {
  const expenses = db.expenses
    .filter((e) => e.month === month)
    .sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0));
  return { month, expenses };
}

// 対象月の精算（settled フラグ付き）。収入未入力なら computeSettlement が INCOME_NOT_READY を投げる。
function settlementOf(db: DemoDb, month: string): Settlement {
  const s = computeSettlement({
    month,
    members: db.members,
    weights: db.weights,
    incomes: db.incomes,
    expenses: db.expenses,
    recurring: db.recurring,
  });
  return { ...s, settled: Boolean(db.settled[month]) };
}

// demoApi は (method, path, body) を実ハンドラ相当のレスポンスへマッピングする。
// path は必要に応じてクエリ文字列を含む（例: /expenses?month=2026-07）。
export async function demoApi(method: HttpMethod, path: string, body?: unknown): Promise<unknown> {
  const [rawPath, queryStr] = path.split("?");
  const q = new URLSearchParams(queryStr || "");
  const b = (body ?? {}) as DemoBody;
  const db = store.get();
  const key = `${method} ${rawPath}`;

  let mm: RegExpMatchArray | null;

  // --- 認証 ---
  if (key === "POST /login") {
    const member = db.members.find((m) => m.id === b.memberId);
    if (!member) unauthorized("メンバーIDが違います（デモでは taro / hanako が使えます）");
    const expiresAt = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString();
    return { token: "demo-token", member: { id: member.id, name: member.name }, expiresAt };
  }

  // --- メンバー ---
  if (key === "GET /members") {
    return { members: db.members.map((m) => ({ id: m.id, name: m.name, color: m.color })) };
  }
  if (method === "PUT" && (mm = rawPath.match(/^\/members\/([^/]+)$/))) {
    const member = db.members.find((m) => m.id === mm![1]);
    if (!member) notFound("メンバーが見つかりません");
    if (b.name != null) member.name = b.name;
    if (b.color != null) member.color = b.color;
    store.save();
    return { id: member.id, name: member.name, color: member.color };
  }

  // --- 支出 ---
  if (key === "POST /expenses") {
    const date = b.date;
    if (!date) validation("日付は必須です");
    if (!b.amountYen || b.amountYen <= 0) validation("金額は1円以上で入力してください");
    const month = date.slice(0, 7);
    const expense = {
      id: `${month}_${randHex()}`,
      paidBy: b.paidBy ?? "",
      amountYen: b.amountYen,
      description: b.description || "",
      date,
      month,
      createdAt: nowISO(),
    };
    db.expenses.push(expense);
    store.save();
    return expense;
  }
  if (key === "GET /expenses") {
    const month = q.get("month");
    if (!month) validation("month は必須です");
    return listExpenses(db, month);
  }
  if (method === "PUT" && (mm = rawPath.match(/^\/expenses\/([^/]+)$/))) {
    const expense = db.expenses.find((e) => e.id === mm![1]);
    if (!expense) notFound("支出が見つかりません");
    if (b.paidBy != null) expense.paidBy = b.paidBy;
    if (b.amountYen != null) expense.amountYen = b.amountYen;
    if (b.description != null) expense.description = b.description;
    if (b.date != null) {
      expense.date = b.date;
      expense.month = b.date.slice(0, 7);
    }
    store.save();
    return expense;
  }
  if (method === "DELETE" && (mm = rawPath.match(/^\/expenses\/([^/]+)$/))) {
    const idx = db.expenses.findIndex((e) => e.id === mm![1]);
    if (idx < 0) notFound("支出が見つかりません");
    db.expenses.splice(idx, 1);
    store.save();
    return null;
  }

  // --- 収入 ---
  if (method === "PUT" && (mm = rawPath.match(/^\/months\/([^/]+)\/incomes\/([^/]+)$/))) {
    const [, month, memberId] = mm;
    if (!db.members.some((m) => m.id === memberId)) validation("不明なメンバーです");
    if (b.amountYen == null || b.amountYen < 0) validation("収入は0以上で入力してください");
    let income = db.incomes.find((i) => i.month === month && i.memberId === memberId);
    if (income) income.amountYen = b.amountYen;
    else {
      income = { month, memberId, amountYen: b.amountYen };
      db.incomes.push(income);
    }
    store.save();
    return { month, income: { memberId, amountYen: income.amountYen } };
  }
  if (method === "GET" && (mm = rawPath.match(/^\/months\/([^/]+)\/incomes$/))) {
    const month = mm[1];
    const incomes = db.incomes
      .filter((i) => i.month === month)
      .map((i) => ({ memberId: i.memberId, amountYen: i.amountYen }));
    return { month, incomes };
  }

  // --- 精算 ---
  if (method === "GET" && (mm = rawPath.match(/^\/months\/([^/]+)\/settlement$/))) {
    return settlementOf(db, mm[1]); // 収入未入力なら INCOME_NOT_READY を投げる
  }
  if (method === "PUT" && (mm = rawPath.match(/^\/months\/([^/]+)\/settlement\/status$/))) {
    const month = mm[1];
    db.settled[month] = Boolean(b.settled);
    store.save();
    return { month, settled: db.settled[month] };
  }

  // --- 精算履歴 ---
  if (key === "GET /settlements/history") {
    const from = q.get("from");
    const to = q.get("to");
    if (!from || !to) validation("from / to は必須です");
    const entries = [];
    // to から from へ（新しい月順）走査し、収入完備の月のみ採用する。
    for (let month = to; monthIndex(month) >= monthIndex(from); month = shiftMonth(month, -1)) {
      let s: Settlement;
      try {
        s = settlementOf(db, month);
      } catch (err) {
        if (err instanceof ApiError && err.code === "INCOME_NOT_READY") continue;
        throw err;
      }
      entries.push({ month: s.month, settled: s.settled, totalExpenseYen: s.totalExpenseYen, transfer: s.transfer });
    }
    return { entries };
  }

  // --- 固定費 ---
  if (key === "GET /recurring-expenses") {
    return {
      recurringExpenses: db.recurring.map((r) => ({
        id: r.id,
        paidBy: r.paidBy,
        amountYen: r.amountYen,
        description: r.description,
      })),
    };
  }
  if (key === "POST /recurring-expenses") {
    if (!b.description) validation("内容は必須です");
    if (!b.amountYen || b.amountYen <= 0) validation("金額は1円以上で入力してください");
    const item = { id: `recurring-${randHex()}`, paidBy: b.paidBy ?? "", amountYen: b.amountYen, description: b.description };
    db.recurring.push(item);
    store.save();
    return item;
  }
  if (method === "PUT" && (mm = rawPath.match(/^\/recurring-expenses\/([^/]+)$/))) {
    const item = db.recurring.find((r) => r.id === mm![1]);
    if (!item) notFound("固定費が見つかりません");
    if (b.paidBy != null) item.paidBy = b.paidBy;
    if (b.amountYen != null) item.amountYen = b.amountYen;
    if (b.description != null) item.description = b.description;
    store.save();
    return item;
  }
  if (method === "DELETE" && (mm = rawPath.match(/^\/recurring-expenses\/([^/]+)$/))) {
    const idx = db.recurring.findIndex((r) => r.id === mm![1]);
    if (idx < 0) notFound("固定費が見つかりません");
    db.recurring.splice(idx, 1);
    store.save();
    return null;
  }

  // --- 精算比重 ---
  if (key === "GET /settings/weight") {
    return { weights: { ...db.weights } };
  }
  if (key === "PUT /settings/weight") {
    const next: Weights = {};
    for (const m of db.members) {
      const w = b.weights?.[m.id];
      if (w == null || w < 1) validation("比重は1以上で入力してください");
      next[m.id] = w;
    }
    db.weights = next;
    store.save();
    return { weights: { ...db.weights } };
  }

  // --- ヘルスチェック（画面からは未使用だが念のため） ---
  if (key === "GET /health") return { status: "ok" };

  notFound(`デモ未対応のエンドポイントです: ${key}`);
}
