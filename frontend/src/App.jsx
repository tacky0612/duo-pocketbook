import { useCallback, useEffect, useState } from "react";
import { api, ApiError } from "./lib/apiClient.js";
import { session } from "./lib/session.js";
import { useTheme } from "./theme.js";
import AppShell from "./components/AppShell.jsx";
import Toast from "./components/Toast.jsx";
import { Spinner } from "./components/ui";
import LoginScreen from "./screens/LoginScreen.jsx";
import SettlementScreen from "./screens/SettlementScreen.jsx";
import IncomeScreen from "./screens/IncomeScreen.jsx";
import ExpenseScreen from "./screens/ExpenseScreen.jsx";
import RecurringScreen from "./screens/RecurringScreen.jsx";
import HistoryScreen from "./screens/HistoryScreen.jsx";
import SettingsScreen from "./screens/SettingsScreen.jsx";

function thisMonth() {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`;
}

export default function App() {
  const theme = useTheme();
  const [me, setMe] = useState(session.member);
  const [members, setMembers] = useState([]);
  const [membersLoading, setMembersLoading] = useState(false);
  const [screen, setScreen] = useState("settlement");
  const [month, setMonth] = useState(thisMonth());
  const [toast, setToast] = useState(null);

  const notify = useCallback((message, kind = "success") => {
    setToast({ message, kind, at: Date.now() });
  }, []);

  const logout = useCallback(() => {
    session.clear();
    setMe(null);
    setMembers([]);
    setScreen("settlement");
  }, []);

  // プロフィール（表示名・カラー）の更新をメンバー一覧と自分の情報へ反映する
  const handleMemberUpdated = useCallback((updated) => {
    setMembers((prev) => prev.map((m) => (m.id === updated.id ? { ...m, ...updated } : m)));
    setMe((prev) => {
      if (prev?.id !== updated.id) return prev;
      const next = { ...prev, name: updated.name };
      session.member = next;
      return next;
    });
  }, []);

  // 401 は自動ログアウト、それ以外はトースト表示
  const handleError = useCallback((err) => {
    if (err instanceof ApiError && err.status === 401) {
      logout();
      notify("認証が切れました。再ログインしてください。", "error");
      return;
    }
    notify(err.message, "error");
  }, [logout, notify]);

  // ログイン済みならメンバー一覧を取得
  useEffect(() => {
    if (!me) return;
    setMembersLoading(true);
    api("GET", "/members")
      .then((res) => setMembers(res.members))
      .catch(handleError)
      .finally(() => setMembersLoading(false));
  }, [me, handleError]);

  if (!me) {
    return (
      <>
        <LoginScreen onLoggedIn={setMe} />
        <Toast toast={toast} />
      </>
    );
  }

  const shared = { month, members, me, notify, onError: handleError, onNavigate: setScreen, onMonthChange: setMonth };

  return (
    <AppShell screen={screen} onNavigate={setScreen} month={month} onMonthChange={setMonth}>
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
