import type { ReactNode } from "react";

interface CardProps {
  children?: ReactNode;
  className?: string;
}

export default function Card({ children, className = "" }: CardProps) {
  return (
    <div
      className={
        "rounded-2xl border border-slate-200/80 bg-white/90 p-5 shadow-sm " +
        "dark:border-slate-800 dark:bg-slate-900/70 " +
        className
      }
    >
      {children}
    </div>
  );
}
