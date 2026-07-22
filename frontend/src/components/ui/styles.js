// UIプリミティブ間で共有する Tailwind クラス文字列。

export const fieldInput =
  "w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-slate-900 " +
  "placeholder:text-slate-400 shadow-sm transition-colors " +
  "focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500/30 " +
  "dark:border-slate-700 dark:bg-slate-950/50 dark:text-slate-100 dark:placeholder:text-slate-500";

export const buttonVariants = {
  primary:
    "bg-blue-600 text-white hover:bg-blue-500 active:bg-blue-700 shadow-sm shadow-blue-600/20",
  secondary:
    "bg-slate-100 text-slate-700 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700",
  danger:
    "bg-transparent text-rose-600 border border-rose-300 hover:bg-rose-50 dark:border-rose-900/60 dark:text-rose-400 dark:hover:bg-rose-950/40",
  ghost:
    "bg-transparent text-slate-500 hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800",
};
