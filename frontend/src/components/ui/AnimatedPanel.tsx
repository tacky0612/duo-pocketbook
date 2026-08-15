import type { ReactNode } from "react";

interface AnimatedPanelProps {
  // これが変わるたびに中身を入れ替え、出現アニメーションを再生する（例: タブのキー）。
  motionKey: string;
  children: ReactNode;
  className?: string;
}

// motionKey の変化に応じて、フェード＋わずかな上方向スライドで中身をしなやかに切り替える。
// key を付け替えることで React が再マウントし、panel-in（index.css）が毎回再生される。
// prefers-reduced-motion では motion-safe が外れてアニメーションしない。
export default function AnimatedPanel({ motionKey, children, className = "" }: AnimatedPanelProps) {
  return (
    <div
      key={motionKey}
      className={"motion-safe:animate-[panel-in_0.45s_cubic-bezier(0.22,1,0.36,1)] " + className}
    >
      {children}
    </div>
  );
}
