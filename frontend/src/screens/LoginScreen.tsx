import { useState, type FormEvent } from "react";
import { api, defaultApiBase, FIXED_API_BASE } from "../lib/apiClient";
import { session } from "../lib/session";
import { Button, Field, Input } from "../components/ui";
import { GlobeIcon, UserIcon, LockIcon, PlayIcon, GitHubIcon } from "../components/Icons";
import Wordmark from "../components/Wordmark";
import type { LoginResponse, Member } from "../types";

interface LoginScreenProps {
  onLoggedIn: (member: Member) => void;
}

export default function LoginScreen({ onLoggedIn }: LoginScreenProps) {
  const [apiBase, setApiBase] = useState(session.apiBase || defaultApiBase());
  const [memberId, setMemberId] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setError(null);
    setBusy(true);
    // API ベースがビルド時に固定されている場合はユーザー入力を保存しない
    if (!FIXED_API_BASE) session.apiBase = apiBase.replace(/\/+$/, "");
    try {
      const data = await api<LoginResponse>("POST", "/login", { memberId: memberId.trim(), password });
      session.token = data.token;
      session.member = data.member;
      onLoggedIn(data.member);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  // API を用意できない人向け。モックデータで全機能を試せるデモモードに入る。
  const loginDemo = async () => {
    setError(null);
    setBusy(true);
    session.demo = true;
    try {
      const data = await api<LoginResponse>("POST", "/login", { memberId: "taro", password: "demo" });
      session.token = data.token;
      session.member = data.member;
      onLoggedIn(data.member);
    } catch (err) {
      session.demo = false;
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  };

  return (
    <div className="ledger-bg relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="relative w-full max-w-sm">
        {/* ブランド */}
        <div className="mb-8 text-center">
          <h1>
            <Wordmark className="text-4xl text-slate-800 dark:text-white" />
          </h1>
          <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">ふたりの家計簿</p>
        </div>

        {/* カード */}
        <form
          onSubmit={submit}
          className="space-y-5 rounded-3xl border border-slate-200 bg-white/95 p-7 shadow-xl shadow-slate-900/10 backdrop-blur-xl dark:border-white/10 dark:bg-slate-900/90 dark:shadow-black/40"
        >
          {!FIXED_API_BASE && (
            <Field label="APIのURL">
              <div className="relative">
                <GlobeIcon className="pointer-events-none absolute left-3.5 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400" />
                <Input
                  type="url"
                  required
                  value={apiBase}
                  onChange={(e) => setApiBase(e.target.value)}
                  placeholder="https://xxxx.lambda-url..."
                  className="pl-11"
                />
              </div>
            </Field>
          )}
          <Field label="メンバーID">
            <div className="relative">
              <UserIcon className="pointer-events-none absolute left-3.5 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400" />
              <Input
                type="text"
                required
                autoComplete="username"
                value={memberId}
                onChange={(e) => setMemberId(e.target.value)}
                className="pl-11"
              />
            </div>
          </Field>
          <Field label="パスワード">
            <div className="relative">
              <LockIcon className="pointer-events-none absolute left-3.5 top-1/2 h-5 w-5 -translate-y-1/2 text-slate-400" />
              <Input
                type="password"
                required
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="pl-11"
              />
            </div>
          </Field>
          {error && (
            <p className="rounded-xl border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-600 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-400">
              {error}
            </p>
          )}
          <Button
            type="submit"
            disabled={busy}
            className="w-full bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500"
          >
            {busy ? "ログイン中..." : "ログイン"}
          </Button>

          {/* デモモード導線（API 不要）。API が固定（VITE_API_BASE 指定）の本番配信では非表示にする */}
          {!FIXED_API_BASE && (
            <>
              <div className="flex items-center gap-3 py-1">
                <span className="h-px flex-1 bg-slate-200 dark:bg-slate-800" />
                <span className="text-xs text-slate-400">または</span>
                <span className="h-px flex-1 bg-slate-200 dark:bg-slate-800" />
              </div>
              <button
                type="button"
                onClick={loginDemo}
                disabled={busy}
                className="flex w-full items-center justify-center gap-2 rounded-xl border border-slate-300 py-2.5 text-sm font-medium text-slate-700 transition-colors hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:border-blue-900 dark:hover:bg-blue-950/40 dark:hover:text-blue-300"
              >
                <PlayIcon className="h-4 w-4" />
                デモモードで試す（API不要）
              </button>
              <p className="text-center text-xs text-slate-400">
                サンプルデータでアプリの全機能を体験できます。データはこの端末内にのみ保存されます。
              </p>
            </>
          )}
        </form>

        {/* GitHub リポジトリへのリンク */}
        <div className="mt-6 text-center">
          <a
            href="https://github.com/tacky0612/duo-pocketbook"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 text-xs font-medium text-slate-400 transition-colors hover:text-slate-600 dark:text-slate-500 dark:hover:text-slate-300"
          >
            <GitHubIcon className="h-4 w-4" />
            GitHub リポジトリ
          </a>
        </div>
      </div>
    </div>
  );
}
