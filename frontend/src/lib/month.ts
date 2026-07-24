// 月・精算月に関する日付ユーティリティ。
// 精算月の算出はバックエンド domain.ClosingDay.SettlementMonth と同一ロジックに保つ。

// todayISO は今日の日付を YYYY-MM-DD（ローカルタイム）で返す。
export function todayISO(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
}

// currentYearMonth は今日のカレンダー上の年月(YYYY-MM)を返す（締め日は考慮しない）。
export function currentYearMonth(): string {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

// settlementMonthOf は日付(YYYY-MM-DD)が属する精算月(YYYY-MM)を返す。
// バックエンド domain.ClosingDay.SettlementMonth と同一ロジック。
// 締め日=1（デフォルト）は暦月どおり。D>=2 は実効締め日 min(D, その月の日数) 以上の日を翌月分にする。
export function settlementMonthOf(dateISO: string, closingDay: number): string {
  const [y, m, d] = dateISO.split("-").map(Number);
  const ym = (yy: number, mm: number) => `${yy}-${String(mm).padStart(2, "0")}`;
  if (!closingDay || closingDay <= 1) return ym(y, m);
  const daysInMonth = new Date(y, m, 0).getDate(); // m は1始まり。翌月0日＝当月末日
  const eff = Math.min(closingDay, daysInMonth);
  if (d >= eff) {
    return m === 12 ? ym(y + 1, 1) : ym(y, m + 1);
  }
  return ym(y, m);
}
