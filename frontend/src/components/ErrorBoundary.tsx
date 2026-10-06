import { Component, type ErrorInfo, type ReactNode } from "react";

interface ErrorBoundaryProps {
  // エラー時に代わりに描画する内容。reset を呼ぶと子の描画をやり直す。
  fallback: (error: Error, reset: () => void) => ReactNode;
  children?: ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

// 子孫の描画中に起きたエラーを受け止め、ツリー全体が消えて真っ白になるのを防ぐ。
// イベントハンドラや非同期処理のエラーは対象外（React の仕様）。
export default class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return { error: error instanceof Error ? error : new Error(String(error)) };
  }

  componentDidCatch(error: unknown, info: ErrorInfo) {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  reset = () => this.setState({ error: null });

  render() {
    if (this.state.error) return this.props.fallback(this.state.error, this.reset);
    return this.props.children;
  }
}
