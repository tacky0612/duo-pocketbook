import { useState } from "react";
import { api } from "../lib/apiClient";
import { yen } from "../lib/format";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Button, Spinner, Empty } from "../components/ui";
import { ArrowRightIcon, CheckIcon, ChevronRight } from "../components/Icons";
import type { HistoryResponse, MemberId, MemberView, ScreenProps, SettlementHistoryEntry, Transfer } from "../types";

const WINDOW_MONTHS = 12;

function thisMonth(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

// month（YYYY-MM）から delta か月ずらした YYYY-MM を返す
function shiftMonth(month: string, delta: number): string {
  const [y, m] = month.split("-").map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

function label(month: string): string {
  const [y, m] = month.split("-").map(Number);
  return `${y}年${m}月`;
}

// 精算完了日時（RFC3339）を「YYYY/M/D 精算」の表示にする。パースできなければ空。
function settledLabel(settledAt: string): string {
  const d = new Date(settledAt);
  if (Number.isNaN(d.getTime())) return "";
  return `${d.getFullYear()}/${d.getMonth() + 1}/${d.getDate()} 精算`;
}

export default function HistoryScreen({ members, onError }: ScreenProps) {
  const to = thisMonth();
  // さかのぼる月数（「もっと見る」で増やす）
  const [span, setSpan] = useState(WINDOW_MONTHS);
  const from = shiftMonth(to, -(span - 1));

  const { loading, data, error } = useAsync<HistoryResponse>(
    () => api<HistoryResponse>("GET", `/settlements/history?from=${from}&to=${to}`),
    [from, to]
  );

  if (error) onError(error);

  const entries = data?.entries ?? [];

  return (
    <div className="space-y-4">
      <Card>
        <SectionTitle>精算履歴</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          精算を完了した月の内容を、完了時点のスナップショットとして表示します。あとから固定費や収支を変更しても、この履歴は変わりません。
        </p>

        {loading && entries.length === 0 ? (
          <Spinner />
        ) : entries.length === 0 ? (
          <Empty>まだ精算を完了した月がありません</Empty>
        ) : (
          <ul className="space-y-3">
            {entries.map((e) => (
              <li key={e.month}>
                <HistoryEntryCard entry={e} members={members} />
              </li>
            ))}
          </ul>
        )}

        <div className="mt-4 flex justify-center">
          <Button variant="secondary" onClick={() => setSpan((s) => s + WINDOW_MONTHS)} disabled={loading}>
            さらに過去を表示
          </Button>
        </div>
      </Card>
    </div>
  );
}

// HistoryEntryCard は1か月分のスナップショットを表示する。
// 初期表示はサマリ（振込額）のみで、タップすると明細の詳細をアコーディオンで開く。
function HistoryEntryCard({ entry, members }: { entry: SettlementHistoryEntry; members: MemberView[] }) {
  const [open, setOpen] = useState(false);
  const memberColor = (id: MemberId) => members.find((m) => m.id === id)?.color;
  // 表示名はスナップショットの内容（完了時点）を優先し、無ければ現在のメンバー名にフォールバックする。
  const memberName = (id: MemberId) =>
    entry.members.find((m) => m.id === id)?.name || members.find((m) => m.id === id)?.name || id;

  const hasDirect = entry.totalDirectTransferYen > 0;

  // 内訳行: from → to amount（振込がなければ zeroLabel を表示）。詳細（明るい背景）で使う。
  // 狭い画面ではラベルと値を行単位で折り返しつつ、値（名前→名前 金額）は塊として改行させない。
  const transferRow = (rowLabel: string, t: Transfer | null, zeroLabel: string) => (
    <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-0.5 text-sm">
      <span className="text-slate-500 dark:text-slate-400">{rowLabel}</span>
      {t ? (
        <span className="inline-flex items-center gap-1.5 whitespace-nowrap font-medium tabular-nums">
          <span>{memberName(t.from)}</span>
          <ArrowRightIcon className="h-4 w-4 shrink-0 text-slate-400" />
          <span>{memberName(t.to)}</span>
          <span className="ml-1">{yen(t.amountYen)}</span>
        </span>
      ) : (
        <span className="font-medium text-slate-400">{zeroLabel}</span>
      )}
    </div>
  );

  // その月に各メンバーが支払った共有費（通常支出 + 固定費）
  const itemsFor = (memberId: MemberId) => entry.expenses.filter((e) => e.paidBy === memberId);

  return (
    <div className="overflow-hidden rounded-2xl border border-slate-200 dark:border-slate-800">
      {/* 精算サマリ（タップで詳細を開閉） */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        className="block w-full bg-gradient-to-br from-emerald-600 to-teal-600 p-5 text-left text-white transition-opacity hover:opacity-95"
      >
        <div className="mb-3 flex items-center justify-between gap-2">
          <span className="text-lg font-bold tabular-nums">{label(entry.month)}</span>
          <span className="flex items-center gap-2">
            <span className="inline-flex items-center gap-1 rounded-full bg-white/20 px-2.5 py-1 text-xs font-semibold">
              <CheckIcon className="h-3.5 w-3.5" />
              {settledLabel(entry.settledAt) || "精算済み"}
            </span>
            <ChevronRight className={"h-5 w-5 text-white/80 transition-transform " + (open ? "rotate-90" : "")} />
          </span>
        </div>

        {entry.transfer ? (
          <div className="flex items-center justify-center gap-3">
            <span className="font-semibold">{memberName(entry.transfer.from)}</span>
            <ArrowRightIcon className="h-5 w-5 text-white/70" />
            <span className="font-semibold">{memberName(entry.transfer.to)}</span>
            <span className="ml-1 text-2xl font-bold tabular-nums">{yen(entry.transfer.amountYen)}</span>
          </div>
        ) : (
          <div className="flex items-center justify-center gap-2">
            <CheckIcon className="h-6 w-6" />
            <span className="font-semibold">振込は不要でした</span>
          </div>
        )}

        {!open && (
          <p className="mt-3 text-center text-xs text-white/70">
            共有支出 {yen(entry.totalExpenseYen)}・タップで内訳を表示
          </p>
        )}
      </button>

      {/* 明細の詳細（アコーディオン） */}
      {open && (
      <div className="space-y-3 border-t border-slate-200 p-4 dark:border-slate-800">
        {/* 内訳: 立替精算がある月は「精算 ＋ 立替精算」を明示する */}
        {hasDirect && (
          <div className="space-y-2 rounded-xl bg-slate-50 p-3 dark:bg-slate-800/50">
            {transferRow("精算（比重按分）", entry.settlementTransfer, "0円")}
            {transferRow("立替精算", entry.directTransfer, "相殺（0円）")}
          </div>
        )}

        <div className="grid gap-3 sm:grid-cols-2">
          {entry.members.map((m) => (
            <div key={m.id} className="rounded-xl bg-slate-50 p-4 dark:bg-slate-800/50">
              <div className="flex items-center justify-between">
                <span className="font-semibold">{m.name}</span>
                <span className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-950/60 dark:text-blue-300">
                  比重 {m.weight}
                </span>
              </div>
              <dl className="mt-3 grid grid-cols-3 gap-2 text-center">
                <div>
                  <dt className="text-xs text-slate-400">収入</dt>
                  <dd className="text-sm font-medium tabular-nums">{yen(m.incomeYen)}</dd>
                </div>
                <div>
                  <dt className="text-xs text-slate-400">立替支出</dt>
                  <dd className="text-sm font-medium tabular-nums">{yen(m.paidExpenseYen)}</dd>
                </div>
                <div>
                  <dt className="text-xs text-slate-400">精算後の可処分</dt>
                  <dd className="text-sm font-bold tabular-nums text-blue-600 dark:text-blue-400">
                    {yen(m.disposableYen)}
                  </dd>
                </div>
              </dl>
            </div>
          ))}
        </div>
        <p className="text-right text-sm text-slate-400">
          共有支出合計 <span className="tabular-nums">{yen(entry.totalExpenseYen)}</span>
        </p>

        {/* メンバー別の共有費一覧 */}
        <div className="grid gap-3 sm:grid-cols-2">
          {entry.members.map((m) => {
            const items = itemsFor(m.id);
            const subtotal = items.reduce((s, e) => s + e.amountYen, 0);
            return (
              <div key={m.id} className="rounded-xl border border-slate-100 p-3 dark:border-slate-800">
                <div className="mb-2 flex items-center justify-between">
                  <h4 className="text-sm font-semibold">{m.name} が支払った共有費</h4>
                  <span className="text-sm font-semibold tabular-nums text-slate-600 dark:text-slate-300">
                    {yen(subtotal)}
                  </span>
                </div>
                {items.length === 0 ? (
                  <Empty>この月の支払いはありません</Empty>
                ) : (
                  <ul className="divide-y divide-slate-100 dark:divide-slate-800">
                    {items.map((e, i) => (
                      <li key={i} className="flex items-center gap-3 py-2">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className="truncate text-sm font-medium">{e.description || "（内容なし）"}</span>
                            {e.recurring && (
                              <span className="shrink-0 rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-950/50 dark:text-amber-300">
                                固定
                              </span>
                            )}
                          </div>
                          {e.date && <div className="text-xs text-slate-400 tabular-nums">{e.date}</div>}
                        </div>
                        <span className="whitespace-nowrap text-sm font-semibold tabular-nums">{yen(e.amountYen)}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            );
          })}
        </div>

        {/* 立替精算の一覧（共有支出とは別の A→B 送金） */}
        {entry.directTransfers.length > 0 && (
          <div className="rounded-xl border border-slate-100 p-3 dark:border-slate-800">
            <h4 className="mb-2 text-sm font-semibold text-slate-500 dark:text-slate-400">立替精算</h4>
            <ul className="divide-y divide-slate-100 dark:divide-slate-800">
              {entry.directTransfers.map((t, i) => (
                <li key={i} className="flex items-center gap-3 py-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm font-medium">{t.description || "（内容なし）"}</span>
                      <span
                        className={
                          "shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium " +
                          (t.recurring
                            ? "bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300"
                            : "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400")
                        }
                      >
                        {t.recurring ? "毎月" : "今月だけ"}
                      </span>
                    </div>
                    <div className="mt-0.5 flex items-center gap-1.5 text-xs text-slate-400">
                      <span style={{ color: memberColor(t.from) }} className="font-medium">
                        {memberName(t.from)}
                      </span>
                      <ArrowRightIcon className="h-3.5 w-3.5" />
                      <span style={{ color: memberColor(t.to) }} className="font-medium">
                        {memberName(t.to)}
                      </span>
                    </div>
                  </div>
                  <span className="whitespace-nowrap text-sm font-semibold tabular-nums">{yen(t.amountYen)}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
      )}
    </div>
  );
}
