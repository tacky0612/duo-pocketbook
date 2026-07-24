import { useState } from "react";
import { api, ApiError } from "../lib/apiClient";
import { yen } from "../lib/format";
import { useAsync } from "../hooks";
import { Card, Spinner, Button, Empty } from "../components/ui";
import { ArrowRightIcon, CheckIcon } from "../components/Icons";
import Celebration from "../components/Celebration";
import type { DirectTransfer, DirectTransfersResponse, Expense, ExpensesResponse, MemberId, RecurringExpense, RecurringExpensesResponse, ScreenProps, Settlement, Transfer } from "../types";

interface SettlementData {
  settlement: Settlement | null;
  settlementError: ApiError | null;
  expenses: Expense[];
  recurring: RecurringExpense[];
  directTransfers: DirectTransfer[];
}

interface SettlementItem {
  id: string;
  description: string;
  amountYen: number;
  date?: string;
  recurring: boolean;
}

export default function SettlementScreen({ month, members, notify, onError, onNavigate }: ScreenProps) {
  const [busy, setBusy] = useState(false);
  const [celebrating, setCelebrating] = useState(false);

  const { loading, data, error, reload } = useAsync<SettlementData>(async () => {
    // 支出・固定費・立替精算は収入の有無に関わらず表示したいので先に取得する。
    const [expensesRes, recurringRes, directRes] = await Promise.all([
      api<ExpensesResponse>("GET", `/expenses?month=${month}`),
      api<RecurringExpensesResponse>("GET", "/recurring-expenses"),
      api<DirectTransfersResponse>("GET", `/direct-transfers?month=${month}`),
    ]);
    // 精算は収入未入力だと 409 になるため、失敗しても他の表示は続ける。
    let settlement: Settlement | null = null;
    let settlementError: ApiError | null = null;
    try {
      settlement = await api<Settlement>("GET", `/months/${month}/settlement`);
    } catch (e) {
      settlementError = e instanceof ApiError ? e : new ApiError(String(e), undefined, 0);
    }
    return {
      settlement,
      settlementError,
      expenses: expensesRes.expenses,
      recurring: recurringRes.recurringExpenses,
      directTransfers: directRes.directTransfers,
    };
  }, [month]);

  if (loading) return <Spinner />;
  if (error) {
    const e = error instanceof ApiError ? error : null;
    if (e?.status === 401) onError(e);
    return (
      <Card className="lg:mx-auto lg:max-w-md">
        <Empty>{error instanceof Error ? error.message : String(error)}</Empty>
      </Card>
    );
  }
  if (!data) return null;

  const { settlement, settlementError } = data;
  const memberName = (id: MemberId) => members.find((m) => m.id === id)?.name || id;
  const memberColor = (id: MemberId) => members.find((m) => m.id === id)?.color;
  const settled = Boolean(settlement?.settled);
  // 立替精算が当月に適用されているか（金額ベース。相殺されても内訳は見せたい）。
  const hasDirect = (settlement?.totalDirectTransferYen ?? 0) > 0;

  // 内訳行: from → to amount（振込がなければ zeroLabel を表示）。
  const transferRow = (label: string, t: Transfer | null, zeroLabel: string) => (
    <div className="flex items-center justify-between gap-3 text-sm">
      <span className="text-white/80">{label}</span>
      {t ? (
        <span className="inline-flex items-center gap-1.5 font-medium tabular-nums">
          <span>{memberName(t.from)}</span>
          <ArrowRightIcon className="h-4 w-4 text-white/70" />
          <span>{memberName(t.to)}</span>
          <span className="ml-1">{yen(t.amountYen)}</span>
        </span>
      ) : (
        <span className="font-medium text-white/70">{zeroLabel}</span>
      )}
    </div>
  );

  const setSettled = async (value: boolean, celebrate: boolean) => {
    setBusy(true);
    try {
      await api("PUT", `/months/${month}/settlement/status`, { settled: value });
      if (celebrate) setCelebrating(true);
      else notify("精算済みを取り消しました");
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  // その月に各メンバーが支払った共有費（通常支出 + 固定費）をまとめる
  const itemsFor = (memberId: MemberId): SettlementItem[] => {
    const oneOff: SettlementItem[] = data.expenses
      .filter((e) => e.paidBy === memberId)
      .map((e) => ({ id: e.id, description: e.description, amountYen: e.amountYen, date: e.date, recurring: false }));
    const recurring: SettlementItem[] = data.recurring
      .filter((e) => e.paidBy === memberId)
      .map((e) => ({ id: "r-" + e.id, description: e.description, amountYen: e.amountYen, recurring: true }));
    return [...oneOff, ...recurring];
  };

  return (
    <div className="space-y-4">
      {/* 精算サマリ + 内訳（上から縦に配置） */}
      <div className="space-y-4">
        {settlement ? (
          <>
            <div
              className={
                "overflow-hidden rounded-2xl p-6 text-white shadow-lg " +
                (settled
                  ? "bg-gradient-to-br from-emerald-600 to-teal-600 shadow-emerald-600/20"
                  : "bg-gradient-to-br from-blue-600 to-indigo-600 shadow-blue-600/20")
              }
            >
              {/* ステータス表示 */}
              <div className="mb-3 flex justify-center">
                {settled ? (
                  <span className="inline-flex items-center gap-1 rounded-full bg-white/20 px-3 py-1 text-xs font-semibold">
                    <CheckIcon className="h-4 w-4" />
                    精算済み
                  </span>
                ) : (
                  <span className="inline-flex items-center rounded-full bg-white/15 px-3 py-1 text-xs font-medium text-white/90">
                    未精算
                  </span>
                )}
              </div>

              {settlement.transfer ? (
                <>
                  <p className="text-center text-sm text-white/80">今月の振込</p>
                  <div className="mt-3 flex items-center justify-center gap-3 text-lg font-semibold">
                    <span>{memberName(settlement.transfer.from)}</span>
                    <ArrowRightIcon className="h-5 w-5 text-white/70" />
                    <span>{memberName(settlement.transfer.to)}</span>
                  </div>
                  <p className="mt-2 text-center text-4xl font-bold tabular-nums">
                    {yen(settlement.transfer.amountYen)}
                  </p>
                  <p className="mt-2 text-center text-xs text-white/80">
                    {hasDirect
                      ? "比重に応じた精算額に、立替精算を加えた合計です"
                      : "振り込むと、ふたりの可処分所得が比重どおりになります"}
                  </p>
                </>
              ) : (
                <div className="flex flex-col items-center py-2">
                  <CheckIcon className="h-10 w-10" />
                  <p className="mt-2 text-lg font-semibold">振込は不要です</p>
                  <p className="text-sm text-white/80">
                    {hasDirect ? "精算分と立替精算が相殺されています 🎉" : "今月はぴったり均衡しています 🎉"}
                  </p>
                </div>
              )}

              {/* 内訳: 立替精算がある月は「精算 ＋ 立替精算」を明示する */}
              {hasDirect && (
                <div className="mt-4 space-y-2 rounded-xl bg-white/10 p-3">
                  {transferRow("精算（比重按分）", settlement.settlementTransfer, "0円")}
                  {transferRow("立替精算", settlement.directTransfer, "相殺（0円）")}
                </div>
              )}

              {/* 精算完了ボタン / 取り消し */}
              <div className="mt-5 flex justify-center">
                {settled ? (
                  <button
                    onClick={() => setSettled(false, false)}
                    disabled={busy}
                    className="rounded-xl bg-white/15 px-4 py-2 text-sm font-medium text-white hover:bg-white/25 disabled:opacity-50"
                  >
                    精算済みを取り消す
                  </button>
                ) : (
                  <button
                    onClick={() => setSettled(true, true)}
                    disabled={busy}
                    className="inline-flex items-center gap-2 rounded-xl bg-white px-6 py-2.5 text-sm font-bold text-blue-700 shadow hover:bg-blue-50 disabled:opacity-50"
                  >
                    <CheckIcon className="h-5 w-5" />
                    {busy ? "処理中..." : "精算を完了する"}
                  </button>
                )}
              </div>
            </div>

            <Card>
              <h3 className="mb-3 text-sm font-semibold text-slate-500 dark:text-slate-400">内訳</h3>
              <div className="grid gap-3 sm:grid-cols-2">
                {settlement.members.map((m) => (
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
              <p className="mt-4 text-right text-sm text-slate-400">
                共有支出合計 <span className="tabular-nums">{yen(settlement.totalExpenseYen)}</span>
              </p>
            </Card>
          </>
        ) : (
          <Card className="text-center">
            <div className="py-4">
              <p className="text-slate-600 dark:text-slate-300">
                {settlementError?.code === "INCOME_NOT_READY"
                  ? "ふたりの収入を入力すると精算額が表示されます。"
                  : settlementError?.message || "精算を計算できませんでした。"}
              </p>
              {settlementError?.code === "INCOME_NOT_READY" && (
                <Button className="mt-4" onClick={() => onNavigate("income")}>
                  収入を入力する
                </Button>
              )}
            </div>
          </Card>
        )}
      </div>

      {/* メンバー別の共有費一覧 */}
      <div className="grid gap-4 sm:grid-cols-2">
        {members.map((m) => {
          const items = itemsFor(m.id);
          const subtotal = items.reduce((s, e) => s + e.amountYen, 0);
          return (
            <Card key={m.id}>
              <div className="mb-3 flex items-center justify-between">
                <h3 className="font-semibold">{m.name} が支払った共有費</h3>
                <span className="text-sm font-semibold tabular-nums text-slate-600 dark:text-slate-300">
                  {yen(subtotal)}
                </span>
              </div>
              {items.length === 0 ? (
                <Empty>この月の支払いはありません</Empty>
              ) : (
                <ul className="divide-y divide-slate-100 dark:divide-slate-800">
                  {items.map((e) => (
                    <li key={e.id} className="flex items-center gap-3 py-2.5">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="truncate text-sm font-medium">
                            {e.description || "（内容なし）"}
                          </span>
                          {e.recurring && (
                            <span className="shrink-0 rounded-full bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-950/50 dark:text-amber-300">
                              固定
                            </span>
                          )}
                        </div>
                        {e.date && <div className="text-xs text-slate-400 tabular-nums">{e.date}</div>}
                      </div>
                      <span className="whitespace-nowrap text-sm font-semibold tabular-nums">
                        {yen(e.amountYen)}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </Card>
          );
        })}
      </div>

      {/* 立替精算の一覧（共有支出とは別の A→B 送金） */}
      {data.directTransfers.length > 0 && (
        <Card>
          <h3 className="mb-3 text-sm font-semibold text-slate-500 dark:text-slate-400">立替精算</h3>
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {data.directTransfers.map((t) => (
              <li key={t.id} className="flex items-center gap-3 py-2.5">
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
        </Card>
      )}

      {celebrating && <Celebration onDone={() => setCelebrating(false)} />}
    </div>
  );
}
