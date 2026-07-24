import type { ReactNode } from "react";
import MonthSelector from "./MonthSelector";
import Wordmark from "./Wordmark";
import { ScaleIcon, DownloadIcon, UploadIcon, RepeatIcon, HistoryIcon, SettingsIcon, FileTextIcon, type IconComponent } from "./Icons";
import type { ScreenName } from "../types";

interface NavItem {
  key: ScreenName;
  label: string;
  icon: IconComponent;
  monthScoped: boolean;
}

export const NAV: NavItem[] = [
  { key: "settlement", label: "精算", icon: ScaleIcon, monthScoped: true },
  { key: "income", label: "収入", icon: DownloadIcon, monthScoped: true },
  { key: "expense", label: "支出", icon: UploadIcon, monthScoped: true },
  { key: "recurring", label: "固定費", icon: RepeatIcon, monthScoped: false },
  { key: "history", label: "履歴", icon: HistoryIcon, monthScoped: false },
  { key: "settings", label: "設定", icon: SettingsIcon, monthScoped: false },
];

interface AppShellProps {
  screen: ScreenName;
  onNavigate: (screen: ScreenName) => void;
  month: string;
  onMonthChange: (month: string) => void;
  children?: ReactNode;
}

export default function AppShell({ screen, onNavigate, month, onMonthChange, children }: AppShellProps) {
  const active = NAV.find((n) => n.key === screen);
  const showMonth = active?.monthScoped;

  return (
    <div className="min-h-screen bg-slate-100 text-slate-900 dark:bg-slate-950 dark:text-slate-100 lg:flex">
      {/* サイドバー（PCのみ）。ビューポート高に固定し、コンテンツが長くても最下部リンクが常に見えるようにする */}
      <aside className="hidden lg:sticky lg:top-0 lg:flex lg:h-screen lg:w-64 lg:shrink-0 lg:flex-col lg:border-r lg:border-slate-200 lg:bg-white dark:lg:border-slate-800 dark:lg:bg-slate-900">
        <div className="flex h-16 shrink-0 items-center border-b border-slate-200 px-6 dark:border-slate-800">
          <Wordmark className="text-xl text-slate-900 dark:text-white" subClassName="text-slate-400" />
        </div>
        <nav className="flex flex-1 flex-col gap-1 overflow-y-auto p-3">
          {NAV.map(({ key, label, icon: Icon }) => {
            const on = key === screen;
            return (
              <button
                key={key}
                onClick={() => onNavigate(key)}
                className={
                  "flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors " +
                  (on
                    ? "bg-blue-50 text-blue-700 dark:bg-blue-950/50 dark:text-blue-300"
                    : "text-slate-500 hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200")
                }
              >
                <Icon className="h-5 w-5" />
                {label}
              </button>
            );
          })}
          {/* APIドキュメント（PC のサイドバー最下部。別タブで開く） */}
          <a
            href={`${import.meta.env.BASE_URL}api-docs.html`}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-auto flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-700 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-slate-200"
          >
            <FileTextIcon className="h-5 w-5" />
            APIドキュメント
          </a>
        </nav>
      </aside>

      {/* メイン列 */}
      <div className="flex min-h-screen flex-1 flex-col">
        {/* ヘッダー */}
        <header className="sticky top-0 z-20 bg-gradient-to-r from-blue-700 to-blue-500 shadow-md shadow-blue-900/10">
          <div className="mx-auto flex h-16 w-full max-w-6xl items-center justify-between gap-3 px-4 lg:px-8">
            {/* モバイル: ブランド / PC: 画面タイトル */}
            <div className="flex items-center gap-2 text-white">
              <Wordmark className="text-xl text-white lg:hidden" subClassName="text-blue-100" />
              <span className="hidden text-lg font-semibold tracking-tight lg:inline">{active?.label}</span>
            </div>
            {showMonth && <MonthSelector month={month} onChange={onMonthChange} />}
          </div>
        </header>

        {/* コンテンツ */}
        <main className="mx-auto w-full max-w-6xl flex-1 px-4 pb-28 pt-5 lg:px-8 lg:pb-12">
          {children}
        </main>
      </div>

      {/* ボトムナビ（モバイルのみ） */}
      <nav className="fixed inset-x-0 bottom-0 z-20 border-t border-slate-200 bg-white/90 backdrop-blur lg:hidden dark:border-slate-800 dark:bg-slate-900/90">
        <div className="mx-auto flex max-w-2xl items-stretch justify-around">
          {NAV.map(({ key, label, icon: Icon }) => {
            const on = key === screen;
            return (
              <button
                key={key}
                onClick={() => onNavigate(key)}
                className={
                  "flex flex-1 flex-col items-center gap-1 py-2.5 text-xs font-medium transition-colors " +
                  (on
                    ? "text-blue-600 dark:text-blue-400"
                    : "text-slate-400 hover:text-slate-600 dark:hover:text-slate-300")
                }
              >
                <Icon className={"h-6 w-6 " + (on ? "scale-110 transition-transform" : "")} />
                {label}
              </button>
            );
          })}
        </div>
      </nav>
    </div>
  );
}
