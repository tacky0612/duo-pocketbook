interface AppErrorFallbackProps {
  error: Error;
}

// アプリ全体の描画に失敗したときの画面。真っ白にせず、原因の調査に使えるエラー内容と再読み込みを出す。
export default function AppErrorFallback({ error }: AppErrorFallbackProps) {
  return (
    <div className="flex min-h-screen items-center justify-center px-4 py-10">
      <div className="w-full max-w-md rounded-2xl border border-slate-200/80 bg-white/90 p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900/70">
        <h1 className="text-lg font-semibold">画面を表示できませんでした</h1>
        <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
          再読み込みしても直らない場合は、下のエラー内容を添えて知らせてください。
        </p>
        <pre className="mt-4 max-h-48 overflow-auto whitespace-pre-wrap break-all rounded-xl bg-slate-100 p-3 text-xs text-slate-700 dark:bg-slate-800 dark:text-slate-300">
          {`${error.name}: ${error.message}\n${navigator.userAgent}`}
        </pre>
        <button
          onClick={() => window.location.reload()}
          className="mt-5 w-full rounded-xl bg-blue-600 px-4 py-2.5 text-sm font-bold text-white shadow hover:bg-blue-700"
        >
          再読み込み
        </button>
      </div>
    </div>
  );
}
