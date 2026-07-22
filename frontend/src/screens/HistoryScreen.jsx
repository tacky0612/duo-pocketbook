import { useState } from "react";
import { api } from "../lib/apiClient.js";
import { yen } from "../lib/format.js";
import { useAsync } from "../hooks.js";
import { Card, SectionTitle, Button, Spinner, Empty } from "../components/ui";
import { ArrowRightIcon, CheckIcon } from "../components/Icons.jsx";

const WINDOW_MONTHS = 12;

function thisMonth() {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

// month（YYYY-MM）から delta か月ずらした YYYY-MM を返す
function shiftMonth(month, delta) {
  const [y, m] = month.split("-").map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

function label(month) {
  const [y, m] = month.split("-").map(Number);
  return `${y}年${m}月`;
}

export default function HistoryScreen({ members, onError, onNavigate, onMonthChange }) {
  const to = thisMonth();
  // さかのぼる月数（「もっと見る」で増やす）
  const [span, setSpan] = useState(WINDOW_MONTHS);
  const from = shiftMonth(to, -(span - 1));

  const { loading, data, error } = useAsync(
    () => api("GET", `/settlements/history?from=${from}&to=${to}`),
    [from, to]
  );

  if (error) onError(error);

  const memberName = (id) => members.find((m) => m.id === id)?.name || id;
  const memberColor = (id) => members.find((m) => m.id === id)?.color;
  const entries = data?.entries ?? [];

  // 行クリックでその月の精算画面へ遷移する
  const openMonth = (month) => {
    onMonthChange(month);
    onNavigate("settlement");
  };

  return (
    <div className="space-y-4">
      <Card>
        <SectionTitle>精算履歴</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          収入を入力した月の精算内容と、精算が完了しているかを一覧で確認できます。
        </p>

        {loading && entries.length === 0 ? (
          <Spinner />
        ) : entries.length === 0 ? (
          <Empty>まだ精算対象の月がありません</Empty>
        ) : (
          <ul className="space-y-2">
            {entries.map((e) => (
              <li key={e.month}>
                <button
                  type="button"
                  onClick={() => openMonth(e.month)}
                  className="flex w-full items-center gap-3 rounded-xl border border-slate-200 p-4 text-left transition-colors hover:border-blue-300 hover:bg-blue-50/50 dark:border-slate-800 dark:hover:border-blue-900 dark:hover:bg-blue-950/20"
                >
                  <div className="w-24 shrink-0 whitespace-nowrap font-semibold tabular-nums">{label(e.month)}</div>

                <div className="min-w-0 flex-1">
                  {e.transfer ? (
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
                      <span
                        className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium text-white"
                        style={{ backgroundColor: memberColor(e.transfer.from) }}
                      >
                        {memberName(e.transfer.from)}
                      </span>
                      <ArrowRightIcon className="h-4 w-4 text-slate-400" />
                      <span
                        className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium text-white"
                        style={{ backgroundColor: memberColor(e.transfer.to) }}
                      >
                        {memberName(e.transfer.to)}
                      </span>
                      <span className="font-semibold tabular-nums">{yen(e.transfer.amountYen)}</span>
                    </div>
                  ) : (
                    <span className="text-sm text-slate-500 dark:text-slate-400">精算不要</span>
                  )}
                  <div className="mt-0.5 text-xs text-slate-400">
                    共有支出 {yen(e.totalExpenseYen)}
                  </div>
                </div>

                  {e.settled ? (
                    <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300">
                      <CheckIcon className="h-3.5 w-3.5" />
                      精算済み
                    </span>
                  ) : (
                    <span className="inline-flex shrink-0 items-center rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-400">
                      未精算
                    </span>
                  )}
                </button>
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
