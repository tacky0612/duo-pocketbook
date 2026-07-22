// バックエンドAPIとの通信クライアント。
import { session } from "./session.js";

export class ApiError extends Error {
  constructor(message, code, status) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

export async function api(method, path, body) {
  const headers = { "Content-Type": "application/json" };
  if (session.token) headers["Authorization"] = `Bearer ${session.token}`;
  const res = await fetch(session.apiBase + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return null;
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    throw new ApiError(
      data?.error?.message || `エラーが発生しました (${res.status})`,
      data?.error?.code,
      res.status
    );
  }
  return data;
}

export function defaultApiBase() {
  // ローカルサーバーから配信されている場合は同一オリジンをデフォルトにする
  if (location.protocol.startsWith("http") && location.hostname === "localhost") {
    return location.origin;
  }
  return "";
}
