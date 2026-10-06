// グラフ（recharts）で共有する定数。
import type { MemberId, MemberView } from "../types";

// アカウントカラーが取れないメンバーの色（slate-400）。
export const FALLBACK_COLOR = "#94a3b8";

// 初期表示のアニメーションを有効にするか。OS の「視差効果を減らす」設定では止める。
export const CHART_ANIMATE = !(window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false);

// メンバーIDからアカウントカラーを引く関数を作る。
export function memberColorOf(members: MemberView[]): (id: MemberId) => string {
  return (id) => members.find((m) => m.id === id)?.color || FALLBACK_COLOR;
}
