import { useState } from "react";
import { api, defaultApiBase } from "../lib/apiClient.js";
import { session } from "../lib/session.js";
import { Button, Field, Input } from "../components/ui";

export default function LoginScreen({ onLoggedIn }) {
  const [apiBase, setApiBase] = useState(session.apiBase || defaultApiBase());
  const [memberId, setMemberId] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);

  const submit = async (ev) => {
    ev.preventDefault();
    setError(null);
    setBusy(true);
    session.apiBase = apiBase.replace(/\/+$/, "");
    try {
      const data = await api("POST", "/login", { memberId: memberId.trim(), password });
      session.token = data.token;
      session.member = data.member;
      onLoggedIn(data.member);
    } catch (err) {
      setError(err.message);
    } finally {
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
        </form>
      </div>
    </div>
  );
}
