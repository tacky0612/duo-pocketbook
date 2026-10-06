// 開発サーバー限定で、エラーバウンダリの表示を確認するためにわざと描画エラーを起こす。
//   ?crash        → アプリ全体のエラー表示（AppErrorFallback）
//   ?crash=chart  → グラフ部分のエラー表示（ChartBoundary）
// 本番ビルドでは import.meta.env.DEV が false に置き換わり、何もしない。

type CrashTarget = "app" | "chart";

function crashTarget(): CrashTarget | null {
  const v = new URLSearchParams(window.location.search).get("crash");
  if (v === null) return null;
  return v === "chart" ? "chart" : "app";
}

export default function DevCrash({ target }: { target: CrashTarget }) {
  if (import.meta.env.DEV && crashTarget() === target) {
    throw new Error(`?crash${target === "chart" ? "=chart" : ""} による動作確認用のエラーです`);
  }
  return null;
}
