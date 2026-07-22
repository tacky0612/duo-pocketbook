// バックエンドAPIとの通信クライアント。
import { session } from "./session";
import type { ApiErrorBody } from "../types";

export type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

export class ApiError extends Error {
  code?: string;
  status: number;

  constructor(message: string, code: string | undefined, status: number) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

export async function api<T = unknown>(method: HttpMethod, path: string, body?: unknown): Promise<T> {
  // デモモード時は Lambda ではなく in-browser のモックへ委譲する。
  // 動的 import なので、デモを起動しないユーザーはこのコードを読み込まない（別チャンク）。
  if (session.demo) {
    const { demoApi } = await import("../demo");
    return demoApi(method, path, body) as Promise<T>;
  }
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (session.token) headers["Authorization"] = `Bearer ${session.token}`;
  const res = await fetch(session.apiBase + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return null as T;
  const data = (await res.json().catch(() => null)) as (ApiErrorBody & T) | null;
  if (!res.ok) {
    throw new ApiError(
      data?.error?.message || `エラーが発生しました (${res.status})`,
      data?.error?.code,
      res.status
    );
  }
  return data as T;
}

export function defaultApiBase(): string {
  // ローカルサーバーから配信されている場合は同一オリジンをデフォルトにする
  if (location.protocol.startsWith("http") && location.hostname === "localhost") {
    return location.origin;
  }
  return "";
}
