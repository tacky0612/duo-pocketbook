import { useCallback, useEffect, useRef, useState, type DependencyList } from "react";

export interface AsyncState<T> {
  loading: boolean;
  data: T | null;
  error: unknown;
}

export interface AsyncResult<T> extends AsyncState<T> {
  reload: () => void;
}

// 非同期取得の状態を管理する小さなフック。
// deps が変わるたびに fn を呼び直す。reload() で手動再取得できる。
export function useAsync<T>(fn: () => Promise<T>, deps: DependencyList): AsyncResult<T> {
  const [state, setState] = useState<AsyncState<T>>({ loading: true, data: null, error: null });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const memoFn = useCallback(fn, deps);

  // 最新のリクエスト番号。reload() は前回の取得を取り消さないため、遅れて返った古い応答
  // （月を切り替える前に始まった再取得など）で新しい結果を上書きしないよう、最新以外は捨てる。
  const latest = useRef(0);

  const run = useCallback(() => {
    const seq = ++latest.current;
    setState((s) => ({ ...s, loading: true }));
    memoFn()
      .then((data) => seq === latest.current && setState({ loading: false, data, error: null }))
      .catch((error) => seq === latest.current && setState({ loading: false, data: null, error }));
    return () => {
      // アンマウントや依存の変化で古くなった取得の結果も捨てる。
      if (seq === latest.current) latest.current++;
    };
  }, [memoFn]);

  useEffect(run, [run]);

  return { ...state, reload: run };
}
