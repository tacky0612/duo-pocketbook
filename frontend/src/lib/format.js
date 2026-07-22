// 表示・入力の整形ユーティリティ（API・UIに依存しない純粋関数）。

// 金額を "12,345円" 形式に整形する。
export function yen(n) {
  return `${Number(n).toLocaleString("ja-JP")}円`;
}

// 全角数字（０-９）を半角に変換する。
export function toHalfWidth(s) {
  return String(s).replace(/[０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0));
}

// 全角・半角混在の入力を半角数字のみの文字列へ正規化する。
export function normalizeDigits(s) {
  return toHalfWidth(s).replace(/[^0-9]/g, "");
}
