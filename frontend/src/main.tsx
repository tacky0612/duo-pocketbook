import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import ErrorBoundary from "./components/ErrorBoundary";
import AppErrorFallback from "./components/AppErrorFallback";
import DevCrash from "./components/DevCrash";
import "./index.css";

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("#root 要素が見つかりません");

ReactDOM.createRoot(rootEl).render(
  <React.StrictMode>
    {/* 描画中のエラーで画面全体が真っ白にならないよう、エラー内容と再読み込みを出す */}
    <ErrorBoundary fallback={(error) => <AppErrorFallback error={error} />}>
      <DevCrash target="app" />
      <App />
    </ErrorBoundary>
  </React.StrictMode>
);
