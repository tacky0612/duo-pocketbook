// ログインセッション（トークン・メンバー・APIベースURL）の localStorage 管理。
import type { Member } from "../types";

export const session = {
  get apiBase(): string {
    return localStorage.getItem("apiBase") || "";
  },
  set apiBase(v: string) {
    localStorage.setItem("apiBase", v);
  },
  get token(): string {
    return localStorage.getItem("token") || "";
  },
  set token(v: string) {
    localStorage.setItem("token", v);
  },
  get member(): Member | null {
    return JSON.parse(localStorage.getItem("member") || "null") as Member | null;
  },
  set member(v: Member | null) {
    localStorage.setItem("member", JSON.stringify(v));
  },
  // デモモード。true の間、API 通信は Lambda ではなく in-browser のモックへ委譲される。
  get demo(): boolean {
    return localStorage.getItem("demo") === "1";
  },
  set demo(v: boolean) {
    if (v) localStorage.setItem("demo", "1");
    else localStorage.removeItem("demo");
  },
  clear(): void {
    localStorage.removeItem("token");
    localStorage.removeItem("member");
    localStorage.removeItem("demo");
  },
};
