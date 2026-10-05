import type { YearMonth } from "../types";

interface FrequencyToggleProps {
  recurring: boolean;
  onChange: (recurring: boolean) => void;
  month: YearMonth;
}

// 頻度の切り替え（対象月のみ / 毎月）。収入・予約の登録フォームと編集フォームで使う。
export default function FrequencyToggle({ recurring, onChange, month }: FrequencyToggleProps) {
  const option = (on: boolean) =>
    "rounded-xl border px-3 py-2 text-sm font-medium transition-colors " +
    (on
      ? "border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-500 dark:bg-blue-950/40 dark:text-blue-300"
      : "border-slate-200 text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800");
  return (
    <div className="grid grid-cols-2 gap-2">
      <button type="button" aria-pressed={!recurring} onClick={() => onChange(false)} className={option(!recurring)}>
        {month} のみ
      </button>
      <button type="button" aria-pressed={recurring} onClick={() => onChange(true)} className={option(recurring)}>
        毎月
      </button>
    </div>
  );
}
