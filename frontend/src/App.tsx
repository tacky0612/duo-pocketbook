import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "./lib/apiClient";
import { currentYearMonth, settlementMonthOf, todayISO } from "./lib/month";
import { session } from "./lib/session";
import { useTheme } from "./theme";
import AppShell from "./components/AppShell";
import Toast from "./components/Toast";
import { Spinner } from "./components/ui";
import LoginScreen from "./screens/LoginScreen";
import SettlementScreen from "./screens/SettlementScreen";
import IncomeScreen from "./screens/IncomeScreen";
import ExpenseScreen from "./screens/ExpenseScreen";
import RecurringScreen from "./screens/RecurringScreen";
import HistoryScreen from "./screens/HistoryScreen";
import SettingsScreen from "./screens/SettingsScreen";
import type { ClosingDayResponse, Member, MembersResponse, MemberView, ScreenName, ScreenProps, ToastKind, ToastMessage } from "./types";

export default function App() {
  const theme = useTheme();
  const [me, setMe] = useState<Member | null>(session.member);
  const [members, setMembers] = useState<MemberView[]>([]);
  const [membersLoading, setMembersLoading] = useState(false);
  const [screen, setScreen] = useState<ScreenName>("settlement");
  // 初期値は暦当月。締め日取得後に「今日が属する精算月」へ補正する（下記 effect）。
  const [month, setMonth] = useState<string>(currentYearMonth());
  // 締め日に基づく初期表示月の補正を一度だけ行うためのフラグ。
  const defaultMonthApplied = useRef(false);
  const [toast, setToast] = useState<ToastMessage | null>(null);

  const notify = useCallback((message: string, kind: ToastKind = "success") => {
    setToast({ message, kind, at: Date.now() });
  }, []);

  const logout = useCallback(() => {
    session.clear();
    setMe(null);
    setMembers([]);
    setScreen("settlement");
    // 次回ログイン時に締め日ベースの初期月補正を再適用できるようリセットする。
    defaultMonthApplied.current = false;
    setMonth(currentYearMonth());
  }, []);

  // スクロールを最上部へ戻す。iOS(WebKit)ではスクロールの実体が documentElement か
  // body かが状況で変わるため、window/documentElement/body すべてをリセットする。
  const scrollToTop = useCallback(() => {
    window.scrollTo(0, 0);
    document.documentElement.scrollTop = 0;
    document.body.scrollTop = 0;
  }, []);

  // 画面切替はまず（前画面の高さがある状態で）スクロールを0にしてから差し替える。
  // 切替後のスピナー表示・レイアウト未確定な状態でリセットするより iOS では確実。
  const navigate = useCallback(
    (next: ScreenName) => {
      scrollToTop();
      setScreen(next);
    },
    [scrollToTop]
  );

  // プロフィール（表示名・カラー）の更新をメンバー一覧と自分の情報へ反映する
  const handleMemberUpdated = useCallback((updated: MemberView) => {
    setMembers((prev) => prev.map((m) => (m.id === updated.id ? { ...m, ...updated } : m)));
    setMe((prev) => {
      if (prev?.id !== updated.id) return prev;
      const next: Member = { ...prev, name: updated.name };
      session.member = next;
      return next;
    });
  }, []);

  // 401 は自動ログアウト、それ以外はトースト表示
  const handleError = useCallback(
    (err: unknown) => {
      if (err instanceof ApiError && err.status === 401) {
        logout();
        notify("認証が切れました。再ログインしてください。", "error");
        return;
      }
      notify(err instanceof Error ? err.message : String(err), "error");
    },
    [logout, notify]
  );

  // ログイン済みならメンバー一覧を取得
  useEffect(() => {
    if (!me) return;
    setMembersLoading(true);
    api<MembersResponse>("GET", "/members")
      .then((res) => setMembers(res.members))
      .catch(handleError)
      .finally(() => setMembersLoading(false));
  }, [me, handleError]);

  // 締め日を取得して初期表示月を「今日が属する精算月」へ補正する。
  // 締め日=1（暦月）なら当月と一致するため実質変化しない。ユーザーが手動で月を
  // 変更済みの場合や取得失敗時は暦当月のままにする（致命的でないため無視）。
  useEffect(() => {
    if (!me || defaultMonthApplied.current) return;
    api<ClosingDayResponse>("GET", "/settings/closing-day")
      .then((res) => {
        defaultMonthApplied.current = true;
        const target = settlementMonthOf(todayISO(), res.closingDay);
        setMonth((cur) => (cur === currentYearMonth() ? target : cur));
      })
      .catch(() => {
        /* 締め日の取得に失敗しても暦当月で続行する */
      });
  }, [me]);

  // 画面が変わったら最上部から表示する保険。navigate 経由でない画面変更（ログアウト等）
  // にも対応する。遷移直後のレイアウト未確定に備えて次フレームでも実行する。
  useEffect(() => {
    scrollToTop();
    const id = requestAnimationFrame(scrollToTop);
    return () => cancelAnimationFrame(id);
  }, [screen, scrollToTop]);

  if (!me) {
    return (
      <>
        <LoginScreen onLoggedIn={setMe} />
        <Toast toast={toast} />
      </>
    );
  }

  const shared: ScreenProps = {
    month,
    members,
    me,
    notify,
    onError: handleError,
    onNavigate: navigate,
    onMonthChange: setMonth,
  };

  return (
    <AppShell screen={screen} onNavigate={navigate} month={month} onMonthChange={setMonth}>
      {membersLoading || members.length === 0 ? (
        <Spinner />
      ) : screen === "settlement" ? (
        <SettlementScreen {...shared} />
      ) : screen === "income" ? (
        <IncomeScreen {...shared} />
      ) : screen === "expense" ? (
        <ExpenseScreen {...shared} />
      ) : screen === "recurring" ? (
        <RecurringScreen {...shared} />
      ) : screen === "history" ? (
        <HistoryScreen {...shared} />
      ) : (
        <SettingsScreen
          {...shared}
          theme={theme}
          onLogout={logout}
          onMemberUpdated={handleMemberUpdated}
        />
      )}
      <Toast toast={toast} />
    </AppShell>
  );
}
