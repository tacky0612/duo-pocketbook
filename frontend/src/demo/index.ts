// デモモードのエントリポイント（バレル）。
// apiClient から動的 import され、Vite により本体とは別チャンクに分割される。
export { demoApi } from "./demoApi";
export { store } from "./store";
