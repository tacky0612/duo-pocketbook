import type { ReactNode } from "react";

export default function Empty({ children }: { children?: ReactNode }) {
  return (
    <p className="py-6 text-center text-sm text-slate-400 dark:text-slate-500">{children}</p>
  );
}
