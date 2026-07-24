import type { CSSProperties } from "react";

interface MemberBadgeProps {
  name: string;
  color?: string;
}

// メンバーを色分けするバッジ（支払者の識別用）。
// color（"#RRGGBB"）が指定されればその色の塗りつぶしピル + 白文字で表示する。
export default function MemberBadge({ name, color }: MemberBadgeProps) {
  const style: CSSProperties | undefined = color ? { backgroundColor: color } : undefined;
  return (
    <span
      className="inline-flex max-w-full items-center truncate rounded-full px-2 py-0.5 text-xs font-medium text-white"
      style={style}
    >
      {name}
    </span>
  );
}
