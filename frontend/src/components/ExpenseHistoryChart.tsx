import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { TooltipContentProps } from "recharts";
import { yen } from "../lib/format";
import { CHART_ANIMATE, memberColorOf } from "../lib/chart";
import ChartLegend from "./ChartLegend";
import type { MemberId, MemberView, SettlementHistoryEntry } from "../types";

interface ExpenseHistoryChartProps {
  // 精算履歴（新しい月順）。
  entries: SettlementHistoryEntry[];
  members: MemberView[];
}

// 1か月分の点。m_<id> にメンバーごとの支払った共有費を持つ。
interface MonthPoint {
  month: string;
  total: number;
  [memberKey: `m_${string}`]: number;
}

const memberKey = (id: MemberId) => `m_${id}` as const;

// "2026-03" → "26/3"（軸ラベル用に短くする）
function shortMonth(month: string): string {
  const [y, m] = month.split("-").map(Number);
  return `${String(y).slice(2)}/${m}`;
}

function fullMonth(month: string): string {
  const [y, m] = month.split("-").map(Number);
  return `${y}年${m}月`;
}

// 縦軸の目盛り。1万円以上は「万」単位で短く表示する。
function axisYen(n: number): string {
  return n >= 10000 ? `${Math.round(n / 1000) / 10}万` : n.toLocaleString("ja-JP");
}

// 精算済みの月ごとに、各メンバーが支払った共有費を積層面グラフで示す。
// 積み上げの上端が共有支出の合計になり、合計と分担の推移を同時に追える。
export default function ExpenseHistoryChart({ entries, members }: ExpenseHistoryChartProps) {
  const colorOf = memberColorOf(members);

  // 履歴は新しい月順なので、左から古い順に並べ替える。
  const data: MonthPoint[] = [...entries].reverse().map((e) => {
    const p: MonthPoint = { month: e.month, total: e.totalExpenseYen };
    for (const m of members) p[memberKey(m.id)] = e.members.find((s) => s.id === m.id)?.paidExpenseYen ?? 0;
    return p;
  });

  return (
    <div>
      <ChartLegend members={members} colorOf={colorOf} />

      <div className="h-56 text-slate-500 dark:text-slate-400">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
            <CartesianGrid vertical={false} stroke="currentColor" strokeOpacity={0.15} />
            <XAxis
              dataKey="month"
              tickFormatter={shortMonth}
              axisLine={false}
              tickLine={false}
              tick={{ fill: "currentColor", fontSize: 11 }}
              minTickGap={12}
            />
            <YAxis
              width={44}
              tickFormatter={axisYen}
              axisLine={false}
              tickLine={false}
              tick={{ fill: "currentColor", fontSize: 11 }}
            />
            <Tooltip
              cursor={{ stroke: "currentColor", strokeOpacity: 0.3 }}
              isAnimationActive={false}
              content={(props) => <HistoryTooltip {...props} members={members} colorOf={colorOf} />}
            />
            {members.map((m) => (
              <Area
                key={m.id}
                type="linear"
                dataKey={memberKey(m.id)}
                name={m.name}
                stackId="expense"
                stroke={colorOf(m.id)}
                strokeWidth={2}
                fill={colorOf(m.id)}
                fillOpacity={0.35}
                activeDot={{ r: 4, strokeWidth: 2 }}
                isAnimationActive={CHART_ANIMATE}
                animationDuration={1000}
                animationEasing="ease-out"
              />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

// ホバーした月の、各メンバーの支払額と合計を示す。
function HistoryTooltip({
  active,
  payload,
  members,
  colorOf,
}: TooltipContentProps & { members: MemberView[]; colorOf: (id: MemberId) => string }) {
  const point = payload?.[0]?.payload as MonthPoint | undefined;
  if (!active || !point) return null;
  return (
    <div className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs shadow-lg dark:border-slate-700 dark:bg-slate-800">
      <p className="mb-1 font-semibold text-slate-700 dark:text-slate-200">{fullMonth(point.month)}</p>
      <ul className="space-y-0.5">
        {members.map((m) => (
          <li key={m.id} className="flex items-center gap-2 text-slate-600 dark:text-slate-300">
            <span className="h-2 w-2 rounded-sm" style={{ backgroundColor: colorOf(m.id) }} />
            <span className="flex-1">{m.name}</span>
            <span className="tabular-nums font-medium">{yen(point[memberKey(m.id)])}</span>
          </li>
        ))}
      </ul>
      <p className="mt-1 flex justify-between gap-4 border-t border-slate-100 pt-1 text-slate-500 dark:border-slate-700 dark:text-slate-400">
        <span>合計</span>
        <span className="tabular-nums font-semibold text-slate-700 dark:text-slate-200">{yen(point.total)}</span>
      </p>
    </div>
  );
}
