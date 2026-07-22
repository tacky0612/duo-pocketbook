import { ChevronLeft, ChevronRight } from "./Icons.jsx";

function shift(month, delta) {
  const [y, m] = month.split("-").map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

function label(month) {
  const [y, m] = month.split("-").map(Number);
  return `${y}年${m}月`;
}

export default function MonthSelector({ month, onChange }) {
  return (
    <div className="flex items-center gap-1 rounded-full bg-white/15 px-1 py-1 backdrop-blur">
      <button
        onClick={() => onChange(shift(month, -1))}
        className="rounded-full p-1.5 text-white/90 hover:bg-white/20"
        aria-label="前の月"
      >
        <ChevronLeft className="h-5 w-5" />
      </button>
      <span className="min-w-[6.5rem] text-center text-sm font-semibold text-white tabular-nums">
        {label(month)}
      </span>
      <button
        onClick={() => onChange(shift(month, 1))}
        className="rounded-full p-1.5 text-white/90 hover:bg-white/20"
        aria-label="次の月"
      >
        <ChevronRight className="h-5 w-5" />
      </button>
    </div>
  );
}
