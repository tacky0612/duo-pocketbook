import { Suspense, type ReactNode } from "react";
import ErrorBoundary from "./ErrorBoundary";
import DevCrash from "./DevCrash";

interface ChartBoundaryProps {
  // 読み込み中に確保しておく領域（レイアウトのずれを防ぐ）。
  placeholder: ReactNode;
  children?: ReactNode;
}

// 遅延読み込みするグラフを包む。グラフの読み込み（別ファイルの取得）や描画に失敗しても、
// グラフ部分だけを差し替えて画面の他の部分は使えるようにする。
export default function ChartBoundary({ placeholder, children }: ChartBoundaryProps) {
  return (
    <ErrorBoundary
      fallback={(error) => (
        <p className="rounded-xl bg-slate-50 p-3 text-center text-xs text-slate-400 dark:bg-slate-800/50">
          グラフを表示できませんでした（{error.message}）
        </p>
      )}
    >
      <Suspense fallback={placeholder}>
        <DevCrash target="chart" />
        {children}
      </Suspense>
    </ErrorBoundary>
  );
}
