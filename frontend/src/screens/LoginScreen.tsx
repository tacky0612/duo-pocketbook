import { useState, type FormEvent } from "react";
import { api, defaultApiBase } from "../lib/apiClient";
import { session } from "../lib/session";
import { Button, Field, Input } from "../components/ui";
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
    session.apiBase = apiBase.replace(/\/+$/, "");
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
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-blue-700 via-blue-600 to-indigo-700 p-4">
      <div className="w-full max-w-sm">
        <div className="mb-8 text-center text-white">
          <h1 className="text-4xl font-bold lowercase tracking-tight">
            duo<span className="font-light text-blue-100">-pocketbook</span>
          </h1>
          <p className="mt-2 text-sm text-blue-100">ふたりの家計簿</p>
        </div>
        <form
          onSubmit={submit}
          className="space-y-4 rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900"
        >
          <Field label="APIのURL">
            <Input
              type="url"
              required
              value={apiBase}
              onChange={(e) => setApiBase(e.target.value)}
              placeholder="https://xxxx.lambda-url..."
            />
          </Field>
          <Field label="メンバーID">
            <Input
              type="text"
              required
              autoComplete="username"
              value={memberId}
              onChange={(e) => setMemberId(e.target.value)}
            />
          </Field>
          <Field label="パスワード">
            <Input
              type="password"
              required
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          {error && (
            <p className="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-600 dark:bg-rose-950/40 dark:text-rose-400">
              {error}
            </p>
          )}
          <Button type="submit" disabled={busy} className="w-full">
            {busy ? "ログイン中..." : "ログイン"}
          </Button>

          {/* デモモード導線（API 不要） */}
          <div className="relative py-1 text-center">
            <span className="relative bg-white px-3 text-xs text-slate-400 dark:bg-slate-900">または</span>
            <span className="absolute inset-x-0 top-1/2 -z-0 border-t border-slate-200 dark:border-slate-800" />
          </div>
          <button
            type="button"
            onClick={loginDemo}
            disabled={busy}
            className="w-full rounded-xl border border-slate-300 py-2.5 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-800"
          >
            デモモードで試す（API不要）
          </button>
          <p className="text-center text-xs text-slate-400">
            サンプルデータでアプリの全機能を体験できます。データはこの端末内にのみ保存されます。
          </p>
        </form>
      </div>
    </div>
  );
}
