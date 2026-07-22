import { useCallback, useEffect, useState, type DependencyList } from "react";

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

  const run = useCallback(() => {
    let alive = true;
    setState((s) => ({ ...s, loading: true }));
    memoFn()
      .then((data) => alive && setState({ loading: false, data, error: null }))
      .catch((error) => alive && setState({ loading: false, data: null, error }));
    return () => {
      alive = false;
    };
  }, [memoFn]);

  useEffect(run, [run]);

  return { ...state, reload: run };
}
