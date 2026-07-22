// テーマの状態管理。モードは "light" | "dark" | "system"。
// "system" のときは OS の設定 (prefers-color-scheme) に追従する。
import { useEffect, useState } from "react";
import type { Theme, ThemeMode } from "./types";

const KEY = "themeMode";

export const THEME_MODES: readonly ThemeMode[] = ["light", "dark", "system"];

function isThemeMode(v: string | null): v is ThemeMode {
  return v !== null && (THEME_MODES as readonly string[]).includes(v);
}

function systemPrefersDark(): boolean {
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false;
}

function resolveDark(mode: ThemeMode): boolean {
  if (mode === "dark") return true;
  if (mode === "light") return false;
  return systemPrefersDark();
}

function apply(dark: boolean): void {
  document.documentElement.classList.toggle("dark", dark);
}

function initialMode(): ThemeMode {
  const saved = localStorage.getItem(KEY);
  return isThemeMode(saved) ? saved : "system";
}

export function useTheme(): Theme {
  const [mode, setMode] = useState<ThemeMode>(initialMode);

  useEffect(() => {
    localStorage.setItem(KEY, mode);
    apply(resolveDark(mode));

    // system のときは OS のテーマ変更にライブで追従する
    if (mode !== "system") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => apply(resolveDark("system"));
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, [mode]);

  return { mode, setMode };
}

// 初回描画時のちらつきを抑えるため即時適用
apply(resolveDark(initialMode()));
