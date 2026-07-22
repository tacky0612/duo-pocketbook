import { useEffect, useState } from "react";
import { CheckIcon } from "./Icons.jsx";

export default function Toast({ toast }) {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    if (!toast) return;
    setVisible(true);
    const timer = setTimeout(() => setVisible(false), 2600);
    return () => clearTimeout(timer);
  }, [toast]);

  if (!toast || !visible) return null;

  const error = toast.kind === "error";
  return (
    <div className="pointer-events-none fixed inset-x-0 bottom-24 z-30 flex justify-center px-4">
      <div
        className={
          "pointer-events-auto flex items-center gap-2 rounded-full px-4 py-2.5 text-sm font-medium text-white shadow-lg " +
          (error ? "bg-rose-600" : "bg-slate-900 dark:bg-blue-600")
        }
      >
        {!error && <CheckIcon className="h-4 w-4" />}
        {toast.message}
      </div>
    </div>
  );
}
