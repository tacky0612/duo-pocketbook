import { Bar, BarChart, LabelList, Rectangle, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import type { BarShapeProps, LabelProps, TooltipContentProps } from "recharts";
import { yen } from "../lib/format";
import { CHART_ANIMATE, memberColorOf } from "../lib/chart";
import ChartLegend from "./ChartLegend";
import type { MemberId, MemberSettlement, MemberView } from "../types";

interface SettlementRatioChartProps {
  members: MemberSettlement[];
  // アカウントカラーの参照元（/members）。
  memberViews: MemberView[];
}

// 1本のバー（収入・支出）。values はメンバーIDごとの金額。
interface RatioRow {
  label: string;
  total: number;
  values: Record<MemberId, number>;
  // 合計0の行（共有費の支払いなしなど）は灰色のトラックだけを描く。
  empty: number;
  [memberKey: `m_${string}`]: number;
}

const RADIUS = 4;
const BAR_SIZE = 22;
const ROW_HEIGHT = 40;

const memberKey = (id: MemberId) => `m_${id}` as const;
const percent = (value: number, total: number) => (total > 0 ? Math.round((value / total) * 100) : 0);

// 精算の内訳を、メンバーごとの比率の100%積み上げ横棒で示す。
export default function SettlementRatioChart({ members, memberViews }: SettlementRatioChartProps) {
  const colorOf = memberColorOf(memberViews);

  const row = (label: string, pick: (m: MemberSettlement) => number): RatioRow => {
    const values: Record<MemberId, number> = {};
    for (const m of members) values[m.id] = Math.max(0, pick(m));
    const total = Object.values(values).reduce((s, v) => s + v, 0);
    const r: RatioRow = { label, total, values, empty: total > 0 ? 0 : 1 };
    for (const m of members) r[memberKey(m.id)] = values[m.id];
    return r;
  };

  const data: RatioRow[] = [
    row("収入", (m) => m.incomeYen),
    row("支出", (m) => m.paidExpenseYen),
  ];

  // 値が0のセグメントを飛ばして、行の左端・右端にあたるセグメントだけ角を丸める。
  const radiusFor = (r: RatioRow, id: MemberId): [number, number, number, number] => {
    const nonZero = members.filter((m) => r.values[m.id] > 0).map((m) => m.id);
    const left = nonZero[0] === id ? RADIUS : 0;
    const right = nonZero[nonZero.length - 1] === id ? RADIUS : 0;
    return [left, right, right, left];
  };

  return (
    <div>
      <ChartLegend members={members} colorOf={colorOf} />

      <div className="text-slate-500 dark:text-slate-400" style={{ height: data.length * ROW_HEIGHT + 8 }}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} layout="vertical" stackOffset="expand" barSize={BAR_SIZE} margin={{ top: 4, right: 4, bottom: 4, left: 0 }}>
            <XAxis type="number" hide domain={[0, 1]} />
            <YAxis
              type="category"
              dataKey="label"
              width={48}
              axisLine={false}
              tickLine={false}
              tick={{ fill: "currentColor", fontSize: 12 }}
            />
            <Tooltip
              cursor={{ fill: "currentColor", fillOpacity: 0.06 }}
              isAnimationActive={false}
              content={(props) => <RatioTooltip {...props} members={members} colorOf={colorOf} />}
            />
            {members.map((m) => (
              <Bar
                key={m.id}
                dataKey={memberKey(m.id)}
                name={m.name}
                stackId="ratio"
                fill={colorOf(m.id)}
                // セグメント間に背景色の隙間を入れて境目を見やすくする。
                className="stroke-white [stroke-width:2px] dark:stroke-slate-900"
                isAnimationActive={CHART_ANIMATE}
                animationDuration={900}
                animationEasing="ease-out"
                shape={(props: BarShapeProps) => (
                  <Rectangle {...props} radius={radiusFor(props.payload as RatioRow, m.id)} />
                )}
              >
                <LabelList
                  dataKey={memberKey(m.id)}
                  content={(props: LabelProps) => <PercentLabel {...props} row={data[Number(props.index)]} id={m.id} />}
                />
              </Bar>
            ))}
            <Bar dataKey="empty" stackId="ratio" fill="currentColor" fillOpacity={0.15} radius={RADIUS} isAnimationActive={false} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

// セグメント内に表示する割合。狭くて収まらないセグメントには出さない。
function PercentLabel({ x, y, width, height, row, id }: LabelProps & { row?: RatioRow; id: MemberId }) {
  const [nx, ny, nw, nh] = [x, y, width, height].map(Number);
  if (!row || row.total <= 0 || !(nw >= 36)) return null;
  return (
    <text
      x={nx + nw / 2}
      y={ny + nh / 2}
      textAnchor="middle"
      dominantBaseline="central"
      fill="#fff"
      stroke="none"
      fontSize={11}
      fontWeight={600}
      className="tabular-nums"
    >
      {percent(row.values[id], row.total)}%
    </text>
  );
}

// ホバー時に、行の各メンバーの実数と割合を示す。
function RatioTooltip({
  active,
  payload,
  members,
  colorOf,
}: TooltipContentProps & { members: MemberSettlement[]; colorOf: (id: MemberId) => string }) {
  const row = payload?.[0]?.payload as RatioRow | undefined;
  if (!active || !row) return null;
  return (
    <div className="rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs shadow-lg dark:border-slate-700 dark:bg-slate-800">
      <p className="mb-1 font-semibold text-slate-700 dark:text-slate-200">{row.label}</p>
      {row.total <= 0 ? (
        <p className="text-slate-500 dark:text-slate-400">ふたりとも0円です</p>
      ) : (
        <ul className="space-y-0.5">
          {members.map((m) => (
            <li key={m.id} className="flex items-center gap-2 text-slate-600 dark:text-slate-300">
              <span className="h-2 w-2 rounded-sm" style={{ backgroundColor: colorOf(m.id) }} />
              <span className="flex-1">{m.name}</span>
              <span className="tabular-nums font-medium">
                {yen(row.values[m.id])}
              </span>
              <span className="w-9 text-right tabular-nums text-slate-400">{percent(row.values[m.id], row.total)}%</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
