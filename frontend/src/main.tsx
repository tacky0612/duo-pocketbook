import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("#root 要素が見つかりません");

ReactDOM.createRoot(rootEl).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
