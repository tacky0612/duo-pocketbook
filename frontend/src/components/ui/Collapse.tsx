import type { ReactNode } from "react";

interface CollapseProps {
  open: boolean;
  children: ReactNode;
  className?: string;
}

// 高さを滑らかにアニメーションするアコーディオン。
// grid-template-rows を 0fr↔1fr で遷移させることで、内容の高さが未知でも
// CSS を書かず Tailwind ユーティリティのみで滑らかに開閉できる。
// 内側の overflow-hidden が閉じている間の中身をクリップする。
// prefers-reduced-motion では即時開閉。
export default function Collapse({ open, children, className = "" }: CollapseProps) {
  return (
    <div
      aria-hidden={!open}
      className={
        "grid transition-[grid-template-rows,opacity] duration-500 ease-out motion-reduce:transition-none " +
        (open ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0") +
        (className ? " " + className : "")
      }
    >
      <div className="overflow-hidden">{children}</div>
    </div>
  );
}
