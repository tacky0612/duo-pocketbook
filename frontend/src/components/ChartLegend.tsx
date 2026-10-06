import type { Member, MemberId } from "../types";

interface ChartLegendProps {
  members: Member[];
  colorOf: (id: MemberId) => string;
}

// グラフの凡例。色だけに頼らずメンバー名を並べて示す。
export default function ChartLegend({ members, colorOf }: ChartLegendProps) {
  return (
    <div className="mb-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-slate-600 dark:text-slate-300">
      {members.map((m) => (
        <span key={m.id} className="inline-flex items-center gap-1.5">
          <span className="h-2.5 w-2.5 rounded-sm" style={{ backgroundColor: colorOf(m.id) }} />
          {m.name}
        </span>
      ))}
    </div>
  );
}
