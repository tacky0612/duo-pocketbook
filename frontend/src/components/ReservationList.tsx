import { useEffect, useRef, useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Field, Input, Select, Button, Spinner, Empty, MemberBadge } from "./ui";
import { TrashIcon, EditIcon } from "./Icons";
import FrequencyToggle from "./FrequencyToggle";
import {
  FrequencyBadge,
  FulfillReservationForm,
  StatusBadge,
  kindLabel,
  memberRoleLabel,
  reservationPlaceholder,
  setReservationSkipped,
} from "./ReservationParts";
import type { MemberId, MemberView, Notify, Reservation, ReservationKind, ReservationsResponse, YearMonth } from "../types";

interface ReservationListProps {
  kind: ReservationKind;
  month: YearMonth;
  members: MemberView[];
  notify: Notify;
  onError: (err: unknown) => void;
  // 金額入力・取り消しで支出／収入の実データが変わったときに呼ぶ（同じ画面の一覧を再取得するため）。
  onActualsChanged?: () => void;
  // 変わるたびに予約一覧を再取得する（予約の追加や、同じ画面での支出／収入の削除を反映するため）。
  refreshKey?: unknown;
  // 締め日。金額入力の日付初期値（精算期間内の日付）に使う。
  closingDay: number;
  // 最初から金額入力フォームを開いておく予約のID。一覧の読み込み後にその行までスクロールする。
  initialFulfillingId?: string;
}

interface ReservationDraft {
  memberId: MemberId;
  description: string;
  recurring: boolean;
}

// 対象月の予約一覧（入力状況つき）。行ごとに金額入力・今月はなし・編集・削除ができる。
// 予約自体は精算に影響せず、金額を入力すると支出／収入として登録されて精算に反映される。
// 金額を入力済みの予約は一覧に出さない（登録された支出／収入は通常の一覧に表示され、それを削除すると未入力に戻る）。
export default function ReservationList({ kind, month, members, notify, onError, onActualsChanged, refreshKey, closingDay, initialFulfillingId }: ReservationListProps) {
  const { loading, data, error, reload } = useAsync<ReservationsResponse>(
    () => api<ReservationsResponse>("GET", `/reservations?month=${month}&kind=${kind}`),
    [month, kind, refreshKey]
  );

  // 行ごとの操作（編集 or 金額入力のどちらか1行だけ開く）
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<ReservationDraft | null>(null);
  const [fulfillingId, setFulfillingId] = useState<string | null>(null);
  const openedInitial = useRef<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (error) onError(error);

  // 精算画面などから特定の予約の金額入力を指定して遷移してきた場合、その予約が未入力なら金額入力を開いて
  // 行を画面内へ移す（初回のみ）。入力済み・今月はなしになっていれば開かない（タブの切り替えで再表示されても同様）。
  useEffect(() => {
    if (!initialFulfillingId || !data || data.month !== month || openedInitial.current === initialFulfillingId) return;
    openedInitial.current = initialFulfillingId;
    const target = data.reservations.find((r) => r.id === initialFulfillingId);
    if (target?.status !== "pending" || data.settled) return;
    setFulfillingId(target.id);
    requestAnimationFrame(() =>
      document.getElementById(`reservation-${target.id}`)?.scrollIntoView({ block: "center", behavior: "smooth" })
    );
  }, [initialFulfillingId, data, month]);

  const memberName = (id: MemberId) => members.find((m) => m.id === id)?.name || id;
  const memberColor = (id: MemberId) => members.find((m) => m.id === id)?.color;
  const label = kindLabel[kind];

  const startEdit = (r: Reservation) => {
    setFulfillingId(null);
    setEditingId(r.id);
    setDraft({
      memberId: r.memberId,
      description: r.description,
      recurring: r.recurring,
    });
  };
  const cancelEdit = () => {
    setEditingId(null);
    setDraft(null);
  };
  const saveEdit = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    if (!draft) return;
    const r0 = list.find((x) => x.id === editingId);
    setBusy(true);
    try {
      // 頻度も変更できる（毎月 ⇄ 表示中の月のみ）。IDは変わらないが、表示する月が変わりうるため一覧を取り直す。
      await api("PUT", `/reservations/${editingId}`, {
        memberId: draft.memberId,
        description: draft.description,
        month: draft.recurring ? "" : month,
        // 毎月のままなら開始月を維持し、単発から毎月へ変えるなら表示中の月から始める。
        startMonth: draft.recurring ? (r0?.recurring ? r0.startMonth : month) : "",
      });
      notify(`${label}の予約を更新しました`);
      cancelEdit();
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const fulfilled = () => {
    setFulfillingId(null);
    reload();
    onActualsChanged?.();
  };

  const toggleSkip = async (r: Reservation, skipped: boolean) => {
    try {
      await setReservationSkipped(r, month, skipped);
      notify(skipped ? `「${r.description}」を今月はなしにしました` : `「${r.description}」を未入力に戻しました`);
      reload();
    } catch (err) {
      onError(err);
    }
  };

  const remove = async (r: Reservation) => {
    if (!confirm(`予約「${r.description}」を削除しますか?`)) return;
    try {
      await api("DELETE", `/reservations/${r.id}`);
      notify("予約を削除しました");
      if (editingId === r.id) cancelEdit();
      reload();
    } catch (err) {
      onError(err);
    }
  };

  const list = data?.reservations ?? [];
  // 金額を入力済みの予約は表示しない。
  const visible = list.filter((r) => r.status !== "fulfilled");
  const fulfilledCount = list.length - visible.length;
  const pendingCount = data?.pendingCount ?? 0;
  // 確定済みの月は金額入力・今月はなし・入力の取り消しができないため、操作ボタンを出さない。
  const settled = data?.settled ?? false;

  return (
    <Card>
      <SectionTitle
        action={
          pendingCount > 0 ? (
            <span className="rounded-full bg-rose-100 px-2 py-0.5 text-xs font-semibold text-rose-700 dark:bg-rose-950/50 dark:text-rose-300">
              未入力 {pendingCount}件
            </span>
          ) : null
        }
      >
        {month} の{label}予約
      </SectionTitle>
      {/* 月を切り替えた直後は前の月の一覧が残っているため表示しない（前の月の予約に対して操作できてしまう） */}
      {settled && data?.month === month && visible.length > 0 && (
        <p className="mb-2 text-xs text-slate-400">この月は精算確定済みのため、金額の入力や「今月はなし」はできません。</p>
      )}
      {(loading && !data) || (data && data.month !== month) ? (
        <Spinner />
      ) : visible.length === 0 ? (
        <Empty>
          {fulfilledCount > 0 ? `この月の${label}の予約はすべて金額を入力済みです` : `この月の${label}の予約はありません`}
        </Empty>
      ) : (
        <ul className="divide-y divide-slate-100 dark:divide-slate-800">
          {visible.map((r) =>
            editingId === r.id && draft ? (
              <li key={r.id} className="py-3">
                {/* インライン編集フォーム（種別は変更不可） */}
                <form
                  onSubmit={saveEdit}
                  className="space-y-3 rounded-xl bg-blue-50/70 p-3 ring-1 ring-blue-200 dark:bg-blue-950/30 dark:ring-blue-900"
                >
                  <Field label={memberRoleLabel[kind]}>
                    <Select
                      value={draft.memberId}
                      onChange={(ev) => setDraft((d) => (d ? { ...d, memberId: ev.target.value } : d))}
                    >
                      {members.map((m) => (
                        <option key={m.id} value={m.id}>{m.name}</option>
                      ))}
                    </Select>
                  </Field>
                  <Field label="内容">
                    <Input
                      type="text" required placeholder={reservationPlaceholder[kind]}
                      value={draft.description}
                      onChange={(ev) => setDraft((d) => (d ? { ...d, description: ev.target.value } : d))}
                    />
                  </Field>
                  <Field
                    label="頻度"
                    hint={
                      draft.recurring !== r.recurring
                        ? draft.recurring
                          ? "他の月にも現れるようになります（入力済みの月はそのまま）。"
                          : `${month} 以外の月には表示されなくなり、「今月はなし」の記録は消えます。`
                        : undefined
                    }
                  >
                    <FrequencyToggle
                      recurring={draft.recurring}
                      onChange={(v) => setDraft((d) => (d ? { ...d, recurring: v } : d))}
                      month={month}
                    />
                  </Field>
                  <div className="flex gap-2">
                    <Button type="button" variant="secondary" onClick={cancelEdit} className="flex-1">
                      キャンセル
                    </Button>
                    <Button type="submit" disabled={busy} className="flex-1">
                      {busy ? "保存中..." : "保存"}
                    </Button>
                  </div>
                </form>
              </li>
            ) : (
              <li key={r.id} id={`reservation-${r.id}`} className="scroll-mt-24 py-3">
                <div className="flex items-center gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className={"truncate font-medium " + (r.status === "skipped" ? "text-slate-400 line-through" : "")}>
                        {r.description}
                      </span>
                      <FrequencyBadge recurring={r.recurring} />
                      <StatusBadge status={r.status} />
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-slate-400">
                      <MemberBadge name={memberName(r.memberId)} color={memberColor(r.memberId)} />
                      {r.recurring && r.startMonth && <span className="tabular-nums">{r.startMonth} から</span>}
                    </div>
                  </div>
                  <button
                    onClick={() => startEdit(r)}
                    className="rounded-lg p-2 text-slate-400 hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-950/40"
                    aria-label="編集"
                  >
                    <EditIcon className="h-5 w-5" />
                  </button>
                  <button
                    onClick={() => remove(r)}
                    className="rounded-lg p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-950/40"
                    aria-label="削除"
                  >
                    <TrashIcon className="h-5 w-5" />
                  </button>
                </div>

                {/* 入力状況に応じた操作 */}
                {fulfillingId === r.id ? (
                  <FulfillReservationForm
                    reservation={r}
                    month={month}
                    closingDay={closingDay}
                    notify={notify}
                    onError={onError}
                    onDone={fulfilled}
                    onCancel={() => setFulfillingId(null)}
                  />
                ) : settled ? null : (
                  <div className="mt-2 flex flex-wrap gap-2">
                    {r.status === "pending" && (
                      <>
                        <Button
                          onClick={() => {
                            cancelEdit();
                            setFulfillingId(r.id);
                          }}
                          className="px-3 py-1.5 text-xs"
                        >
                          金額を入力
                        </Button>
                        {/* 「今月はなし」は毎月の予約のみ。今月だけの予約が不要なら削除する */}
                        {r.recurring && (
                          <Button variant="secondary" onClick={() => toggleSkip(r, true)} className="px-3 py-1.5 text-xs">
                            今月はなし
                          </Button>
                        )}
                      </>
                    )}
                    {r.status === "skipped" && (
                      <Button variant="secondary" onClick={() => toggleSkip(r, false)} className="px-3 py-1.5 text-xs">
                        「今月はなし」を取り消す
                      </Button>
                    )}
                  </div>
                )}
              </li>
            )
          )}
        </ul>
      )}
    </Card>
  );
}
