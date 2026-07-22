import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { session } from "../lib/session";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Field, Input, NumberInput, Button, Spinner } from "../components/ui";
import { SunIcon, MoonIcon, SettingsIcon, LogoutIcon, type IconComponent } from "../components/Icons";
import type { MemberView, ScreenProps, Theme, ThemeMode, WeightsResponse } from "../types";

interface SettingsScreenProps extends ScreenProps {
  theme: Theme;
  onLogout: () => void;
  onMemberUpdated: (member: MemberView) => void;
}

interface ThemeOption {
  value: ThemeMode;
  label: string;
  Icon: IconComponent;
}

const THEME_OPTIONS: ThemeOption[] = [
  { value: "light", label: "ライト", Icon: SunIcon },
  { value: "dark", label: "ダーク", Icon: MoonIcon },
  { value: "system", label: "自動", Icon: SettingsIcon },
];

// アカウントカラーの選択肢（白文字が読みやすい中間トーン）
const COLOR_PALETTE = [
  "#2563eb", "#4f46e5", "#0ea5e9", "#0d9488",
  "#16a34a", "#ca8a04", "#ea580c", "#e11d48",
  "#db2777", "#9333ea", "#64748b", "#0f766e",
];

interface ColorSwatchesProps {
  value?: string;
  onSelect: (color: string) => void;
}

function ColorSwatches({ value, onSelect }: ColorSwatchesProps) {
  return (
    <div className="flex flex-wrap gap-2">
      {COLOR_PALETTE.map((c) => {
        const on = value?.toLowerCase() === c;
        return (
          <button
            key={c}
            type="button"
            onClick={() => onSelect(c)}
            aria-label={c}
            aria-pressed={on}
            className={
              "h-9 w-9 rounded-full transition-transform " +
              (on
                ? "scale-110 ring-2 ring-offset-2 ring-slate-400 dark:ring-offset-slate-900"
                : "hover:scale-105")
            }
            style={{ backgroundColor: c }}
          />
        );
      })}
    </div>
  );
}

interface ThemeSegmentedProps {
  mode: ThemeMode;
  onChange: (mode: ThemeMode) => void;
}

function ThemeSegmented({ mode, onChange }: ThemeSegmentedProps) {
  return (
    <div className="grid grid-cols-3 gap-1 rounded-xl bg-slate-100 p-1 dark:bg-slate-800">
      {THEME_OPTIONS.map(({ value, label, Icon }) => {
        const on = mode === value;
        return (
          <button
            key={value}
            type="button"
            onClick={() => onChange(value)}
            aria-pressed={on}
            className={
              "flex flex-col items-center gap-1 rounded-lg py-2.5 text-xs font-medium transition-colors " +
              (on
                ? "bg-white text-blue-600 shadow-sm dark:bg-slate-950 dark:text-blue-400"
                : "text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200")
            }
          >
            <Icon className="h-5 w-5" />
            {label}
          </button>
        );
      })}
    </div>
  );
}

export default function SettingsScreen({ members, me, theme, notify, onError, onLogout, onMemberUpdated }: SettingsScreenProps) {
  const { loading, data, error, reload } = useAsync<WeightsResponse>(() => api<WeightsResponse>("GET", "/settings/weight"), []);
  const [weights, setWeights] = useState<Record<string, string>>({});
  const [name, setName] = useState(me?.name || "");
  const [savingName, setSavingName] = useState(false);
  const [savingColor, setSavingColor] = useState(false);
  const [busy, setBusy] = useState(false);

  const myColor = members.find((m) => m.id === me?.id)?.color;

  useEffect(() => {
    if (!data) return;
    const next: Record<string, string> = {};
    for (const m of members) next[m.id] = String(data.weights[m.id] ?? 1);
    setWeights(next);
  }, [data, members]);

  useEffect(() => setName(me?.name || ""), [me]);

  if (error) onError(error);

  const saveName = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setSavingName(true);
    try {
      const updated = await api<MemberView>("PUT", `/members/${me.id}`, { name: name.trim() });
      notify("表示名を保存しました");
      onMemberUpdated(updated);
    } catch (err) {
      onError(err);
    } finally {
      setSavingName(false);
    }
  };

  const saveColor = async (color: string) => {
    setSavingColor(true);
    try {
      const updated = await api<MemberView>("PUT", `/members/${me.id}`, { color });
      notify("カラーを保存しました");
      onMemberUpdated(updated);
    } catch (err) {
      onError(err);
    } finally {
      setSavingColor(false);
    }
  };

  const saveWeights = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      const body: Record<string, number> = {};
      for (const m of members) body[m.id] = Number(weights[m.id]);
      await api("PUT", "/settings/weight", { weights: body });
      notify("精算比重を保存しました");
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  // デモモードのデータを初期状態へ戻す。全画面を確実に再取得させるためリロードする。
  const resetDemo = async () => {
    if (!confirm("デモデータを初期状態にリセットしますか?")) return;
    const { store } = await import("../demo");
    store.reset();
    location.reload();
  };

  return (
    <div className="grid gap-4 lg:grid-cols-2 lg:items-start">
      {/* プロフィール（表示名・カラー） */}
      <Card>
        <SectionTitle>プロフィール</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          アプリ内で表示されるあなたの名前とカラーです。
        </p>
        <form onSubmit={saveName} className="space-y-4">
          <Field label="あなたの表示名" hint="20文字以内">
            <Input
              type="text"
              required
              maxLength={20}
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例: 太郎"
            />
          </Field>
          <Button type="submit" disabled={savingName || !name.trim()} className="w-full">
            {savingName ? "保存中..." : "表示名を保存"}
          </Button>
        </form>

        <div className="mt-6 border-t border-slate-200 pt-5 dark:border-slate-800">
          <div className="mb-3 flex items-center justify-between">
            <span className="text-sm font-medium text-slate-600 dark:text-slate-300">アカウントカラー</span>
            <span
              className="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium text-white"
              style={{ backgroundColor: myColor || "#2563eb" }}
            >
              {name || me?.name}
            </span>
          </div>
          <p className="mb-3 text-xs text-slate-400">
            支出一覧の「支払った人」の表示色に使われます。{savingColor && "（保存中...）"}
          </p>
          <ColorSwatches value={myColor} onSelect={saveColor} />
        </div>
      </Card>

      {/* 精算比重 */}
      <Card>
        <SectionTitle>精算比重</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          比重が大きい人ほど、精算後の可処分所得が多く残ります（例 1:1 で均等）。
        </p>
        {loading ? (
          <Spinner />
        ) : (
          <form onSubmit={saveWeights} className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              {members.map((m) => (
                <Field key={m.id} label={`${m.name} の比重`}>
                  <NumberInput
                    required
                    value={weights[m.id] ?? ""}
                    onChange={(v) => setWeights((p) => ({ ...p, [m.id]: v }))}
                    className="text-center tabular-nums"
                  />
                </Field>
              ))}
            </div>
            <Button type="submit" disabled={busy} className="w-full">
              {busy ? "保存中..." : "比重を保存"}
            </Button>
          </form>
        )}
      </Card>

      {/* テーマ */}
      <Card>
        <SectionTitle>テーマ</SectionTitle>
        <ThemeSegmented mode={theme.mode} onChange={theme.setMode} />
        <p className="mt-3 text-xs text-slate-400">
          「自動」を選ぶと端末（OS）の設定に合わせて切り替わります。
        </p>
      </Card>

      {/* アカウント */}
      <Card>
        <SectionTitle>アカウント</SectionTitle>
        <dl className="space-y-2 text-sm">
          <div className="flex justify-between">
            <dt className="text-slate-400">ログイン中</dt>
            <dd className="font-medium">{me?.name}（{me?.id}）</dd>
          </div>
          <div className="flex justify-between gap-4">
            <dt className="shrink-0 text-slate-400">API</dt>
            <dd className="truncate font-mono text-xs text-slate-500">
              {session.demo ? "デモモード（モック）" : session.apiBase || "—"}
            </dd>
          </div>
        </dl>
        {session.demo && (
          <div className="mt-4 rounded-xl bg-amber-50 p-3 dark:bg-amber-950/30">
            <p className="text-xs text-amber-700 dark:text-amber-300">
              デモモードで動作中です。編集内容はこの端末にのみ保存されます。
            </p>
            <Button variant="secondary" onClick={resetDemo} className="mt-3 w-full">
              デモデータをリセット
            </Button>
          </div>
        )}
        <Button variant="danger" onClick={onLogout} className="mt-4 w-full">
          <LogoutIcon className="h-5 w-5" />
          ログアウト
        </Button>
      </Card>
    </div>
  );
}
