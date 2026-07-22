import { useCallback, useEffect, useState } from "react";

// 非同期取得の状態を管理する小さなフック。
// deps が変わるたびに fn を呼び直す。reload() で手動再取得できる。
export function useAsync(fn, deps) {
  const [state, setState] = useState({ loading: true, data: null, error: null });
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
