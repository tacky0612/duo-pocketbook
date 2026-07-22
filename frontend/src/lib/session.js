// ログインセッション（トークン・メンバー・APIベースURL）の localStorage 管理。

export const session = {
  get apiBase() { return localStorage.getItem("apiBase") || ""; },
  set apiBase(v) { localStorage.setItem("apiBase", v); },
  get token() { return localStorage.getItem("token") || ""; },
  set token(v) { localStorage.setItem("token", v); },
  get member() { return JSON.parse(localStorage.getItem("member") || "null"); },
  set member(v) { localStorage.setItem("member", JSON.stringify(v)); },
  clear() {
    localStorage.removeItem("token");
    localStorage.removeItem("member");
  },
};
