import type { ReactNode } from "react";

interface SectionTitleProps {
  children?: ReactNode;
  action?: ReactNode;
}

export default function SectionTitle({ children, action }: SectionTitleProps) {
  return (
    <div className="mb-4 flex items-center justify-between">
      <h2 className="text-lg font-semibold tracking-tight text-slate-900 dark:text-slate-100">
        {children}
      </h2>
      {action}
    </div>
  );
}
