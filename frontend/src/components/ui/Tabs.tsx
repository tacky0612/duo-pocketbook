import type { ReactNode } from "react";

export interface TabItem<T extends string> {
  key: T;
  label: ReactNode;
}

interface TabsProps<T extends string> {
  tabs: TabItem<T>[];
  value: T;
  onChange: (key: T) => void;
  className?: string;
}

// アニメーション付きのセグメント型タブ。
// アクティブなピルが選択位置へ「しなやかに」（ease-out-expo）スライドする。
// CSS は書かず Tailwind ユーティリティのみで実現し、prefers-reduced-motion では即時切替。
export default function Tabs<T extends string>({ tabs, value, onChange, className = "" }: TabsProps<T>) {
  const count = tabs.length;
  const activeIndex = Math.max(0, tabs.findIndex((t) => t.key === value));

  return (
    <div
      role="tablist"
      className={"relative grid rounded-2xl bg-slate-200/70 p-1 dark:bg-slate-800/70 " + className}
      style={{ gridTemplateColumns: `repeat(${count}, minmax(0, 1fr))` }}
    >
      {/* 選択位置へスライドするアクティブ表示（ピル）。p-1（0.5rem）分を差し引いた幅で等分する。 */}
      <span
        aria-hidden
        className="pointer-events-none absolute inset-y-1 left-1 rounded-xl bg-white shadow-sm transition-transform duration-500 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none dark:bg-slate-700"
        style={{
          width: `calc((100% - 0.5rem) / ${count})`,
          transform: `translateX(${activeIndex * 100}%)`,
        }}
      />
      {tabs.map((t) => {
        const on = t.key === value;
        return (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={on}
            onClick={() => onChange(t.key)}
            className={
              "relative z-10 rounded-xl px-3 py-2 text-sm font-semibold transition-colors duration-300 " +
              (on
                ? "text-blue-700 dark:text-blue-300"
                : "text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200")
            }
          >
            {t.label}
          </button>
        );
      })}
    </div>
  );
}
