// デモモードのインメモリ可変ストア。
//
// localStorage（キー demo:db）から復元し、無ければ mockData でシードする。
// 変更系 API はこの db を直接書き換えて save() で永続化するため、リロードしても
// 編集内容が維持される。reset() で初期状態へ戻せる。

import { seedData } from "./mockData";
import type { DemoDb } from "../types";

const KEY = "demo:db";

let db: DemoDb | null = null;

function persist(): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(db));
  } catch {
    // localStorage が使えない環境では永続化を諦める（デモ動作は継続）
  }
}

function load(): DemoDb {
  if (db) return db;
  try {
    const raw = localStorage.getItem(KEY);
    if (raw) {
      db = JSON.parse(raw) as DemoDb;
      return db;
    }
  } catch {
    // 破損データは無視してシードし直す
  }
  db = seedData();
  persist();
  return db;
}

export const store = {
  // 現在のデモDBを返す（未ロードならロード）。
  get(): DemoDb {
    return load();
  },
  // 変更後の永続化。
  save(): void {
    persist();
  },
  // 初期状態へ戻す。
  reset(): DemoDb {
    db = seedData();
    persist();
    return db;
  },
};
