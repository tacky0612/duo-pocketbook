/// <reference types="vite/client" />

interface ImportMetaEnv {
  // 正規クライアント識別用の事前共有キー（ビルド時に注入）。
  // 未設定ならヘッダを送らない（後方互換）。公開ビルドに含まれるため秘密情報ではない。
  readonly VITE_CLIENT_KEY?: string;
  // API のベースURL/パスを固定する（例: 同一オリジン配下の "/api"）。
  // 設定するとログイン画面の「APIのURL」入力は不要になり、この値を使う。
  // 未設定なら従来どおりユーザーがログイン画面で入力する。
  readonly VITE_API_BASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
