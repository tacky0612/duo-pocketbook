import type { ReactNode } from "react";

interface FieldProps {
  label: ReactNode;
  hint?: ReactNode;
  children?: ReactNode;
}

export default function Field({ label, hint, children }: FieldProps) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-sm font-medium text-slate-600 dark:text-slate-300">
        {label}
      </span>
      {children}
      {hint && <span className="mt-1 block text-xs text-slate-400">{hint}</span>}
    </label>
  );
}
