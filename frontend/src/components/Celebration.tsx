import { useEffect, useMemo } from "react";

const COLORS = ["#2563eb", "#4f46e5", "#0ea5e9", "#22c55e", "#f59e0b", "#ec4899"];

interface CelebrationProps {
  onDone: () => void;
}

interface Piece {
  key: number;
  left: number;
  delay: number;
  duration: number;
  size: number;
  color: string;
  rounded: boolean;
}

// 精算完了時に表示する紙吹雪＋メッセージのオーバーレイ。
// 一定時間後、またはクリックで onDone を呼ぶ。
export default function Celebration({ onDone }: CelebrationProps) {
  // 紙吹雪の各片のスタイルを一度だけ生成する
  const pieces = useMemo<Piece[]>(
    () =>
      Array.from({ length: 90 }, (_, i) => ({
        key: i,
        left: Math.random() * 100,
        delay: Math.random() * 0.6,
        duration: 2.2 + Math.random() * 1.6,
        size: 6 + Math.random() * 8,
        color: COLORS[i % COLORS.length],
        rounded: Math.random() > 0.5,
      })),
    []
  );

  useEffect(() => {
    const timer = setTimeout(onDone, 3600);
    return () => clearTimeout(timer);
  }, [onDone]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/40 backdrop-blur-sm"
      onClick={onDone}
    >
      {/* 紙吹雪 */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        {pieces.map((p) => (
          <span
            key={p.key}
            className="confetti-piece"
            style={{
              left: `${p.left}%`,
              width: `${p.size}px`,
              height: `${p.size * 1.4}px`,
              background: p.color,
              borderRadius: p.rounded ? "9999px" : "2px",
              animationDelay: `${p.delay}s`,
              animationDuration: `${p.duration}s`,
            }}
          />
        ))}
      </div>

      {/* メッセージ */}
      <div className="celebrate-pop mx-4 rounded-3xl bg-white px-10 py-8 text-center shadow-2xl dark:bg-slate-900">
        <div className="text-6xl">🎉</div>
        <p className="mt-4 text-2xl font-bold text-slate-900 dark:text-slate-100">
          今月もお疲れさまでした！
        </p>
        <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
          精算が完了しました
        </p>
      </div>
    </div>
  );
}
