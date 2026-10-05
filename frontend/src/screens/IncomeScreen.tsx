import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { yen } from "../lib/format";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Field, Input, NumberInput, Select, Button, Spinner, Empty, MemberBadge, Tabs } from "../components/ui";
import { PlusIcon, TrashIcon, EditIcon } from "../components/Icons";
import FrequencyToggle from "../components/FrequencyToggle";
import ReservationList from "../components/ReservationList";
import { FrequencyBadge, ReservedBadge } from "../components/ReservationParts";
import type { Income, IncomesResponse, MemberId, SalariesResponse, ScreenProps } from "../types";

// 収入の登録方法。actual=金額が決まった通常の収入、reservation=金額未定の予約。
type EntryMode = "actual" | "reservation";

interface IncomeDraft {
  memberId: MemberId;
  amount: string;
  description: string;
}

export default function IncomeScreen({ month, members, me, notify, onError, closingDay }: ScreenProps) {
  // --- 給与（メンバーごと・月ごとに1件） ---
  const salaries = useAsync<SalariesResponse>(
    () => api<SalariesResponse>("GET", `/months/${month}/salaries`),
    [month]
  );
  const [salaryValues, setSalaryValues] = useState<Record<string, string>>({});
  const [savingSalary, setSavingSalary] = useState(false);

  useEffect(() => {
    if (!salaries.data) return;
    const next: Record<string, string> = {};
    for (const m of members) {
      const s = salaries.data.salaries.find((i) => i.memberId === m.id);
      next[m.id] = s != null ? String(s.amountYen) : "";
    }
    setSalaryValues(next);
  }, [salaries.data, members]);

  // --- 追加収入（複数件・単発/継続） ---
  const incomes = useAsync<IncomesResponse>(
    () => api<IncomesResponse>("GET", `/incomes?month=${month}`),
    [month]
  );
  const [memberId, setMemberId] = useState<MemberId>(me?.id || "");
  const [amount, setAmount] = useState("");
  const [description, setDescription] = useState("");
  const [recurring, setRecurring] = useState(true);
  const [mode, setMode] = useState<EntryMode>("actual");
  // 予約一覧を再取得させるためのカウンタ。予約の登録時と、予約から登録した収入を
  // 収入一覧で編集・削除したとき（予約の入力状況が変わる）だけ増やす。
  const [reservationVersion, setReservationVersion] = useState(0);
  const refreshReservations = () => setReservationVersion((v) => v + 1);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<IncomeDraft | null>(null);
  const [busy, setBusy] = useState(false);

  if (salaries.error) onError(salaries.error);
  if (incomes.error) onError(incomes.error);

  const memberName = (id: MemberId) => members.find((m) => m.id === id)?.name || id;
  const memberColor = (id: MemberId) => members.find((m) => m.id === id)?.color;
  const selectedMemberId = memberId || members[0]?.id || "";

  const saveSalaries = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setSavingSalary(true);
    try {
      for (const m of members) {
        const v = salaryValues[m.id];
        if (v !== "" && v != null) {
          await api("PUT", `/months/${month}/salaries/${m.id}`, { amountYen: Number(v) });
        }
      }
      notify("給与を保存しました");
      salaries.reload();
    } catch (err) {
      onError(err);
    } finally {
      setSavingSalary(false);
    }
  };

  // 同じフォームから、通常の収入（/incomes）か予約（/reservations）として登録する。
  // 予約の金額欄は見込み額（任意）として扱う。
  const add = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      const frequency = recurring ? "" : month;
      if (mode === "actual") {
        await api("POST", "/incomes", {
          memberId: selectedMemberId,
          amountYen: Number(amount),
          description,
          month: frequency,
        });
        notify("収入を登録しました");
        incomes.reload();
      } else {
        await api("POST", "/reservations", {
          kind: "income",
          memberId: selectedMemberId,
          description,
          estimatedAmountYen: Number(amount || 0),
          month: frequency,
          // 毎月の予約は表示中の月から始める（登録より前の月に未入力として現れないように）。
          startMonth: recurring ? month : "",
        });
        notify("収入の予約を登録しました");
        refreshReservations();
      }
      setAmount("");
      setDescription("");
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const startEdit = (inc: Income) => {
    setEditingId(inc.id);
    setDraft({ memberId: inc.memberId, amount: String(inc.amountYen), description: inc.description });
  };
  const cancelEdit = () => {
    setEditingId(null);
    setDraft(null);
  };
  const saveEdit = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    if (!draft) return;
    const reserved = Boolean(list.find((i) => i.id === editingId)?.reservationId);
    setBusy(true);
    try {
      await api("PUT", `/incomes/${editingId}`, {
        memberId: draft.memberId,
        amountYen: Number(draft.amount),
        description: draft.description,
      });
      notify("収入を更新しました");
      cancelEdit();
      incomes.reload();
      if (reserved) refreshReservations(); // 予約の入力額が変わる
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (inc: Income) => {
    if (!confirm(`収入「${inc.description}」を削除しますか?`)) return;
    try {
      await api("DELETE", `/incomes/${inc.id}`);
      notify("収入を削除しました");
      if (editingId === inc.id) cancelEdit();
      incomes.reload();
      if (inc.reservationId) refreshReservations(); // 予約が未入力に戻る
    } catch (err) {
      onError(err);
    }
  };

  const list = incomes.data?.incomes ?? [];

  return (
    <div className="mx-auto w-full max-w-3xl space-y-4">
      {/* 給与 */}
      <Card>
        <SectionTitle>給与</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          毎月発生するふたりの基本の収入（手取り）です。精算額の計算に使われます。
        </p>
        {salaries.loading ? (
          <Spinner />
        ) : (
          <form onSubmit={saveSalaries} className="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              {members.map((m) => (
                <Field key={m.id} label={`${m.name} の給与`}>
                  <div className="relative">
                    <NumberInput
                      placeholder="0"
                      value={salaryValues[m.id] ?? ""}
                      onChange={(v) => setSalaryValues((p) => ({ ...p, [m.id]: v }))}
                      className="pr-10 text-right tabular-nums"
                    />
                    <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-slate-400">
                      円
                    </span>
                  </div>
                </Field>
              ))}
            </div>
            <Button type="submit" disabled={savingSalary} className="w-full sm:w-auto">
              {savingSalary ? "保存中..." : "給与を保存"}
            </Button>
          </form>
        )}
      </Card>

      {/* 追加収入 */}
      <div className="grid gap-4 lg:grid-cols-5 lg:items-start">
        <Card className="lg:col-span-2 lg:sticky lg:top-20">
          <SectionTitle>収入を追加</SectionTitle>
          <form onSubmit={add} className="space-y-4">
            {/* 登録方法の切り替え。金額が決まっていれば通常の収入、未定なら予約として登録する。 */}
            <Tabs<EntryMode>
              tabs={[
                { key: "actual", label: "通常の収入" },
                { key: "reservation", label: "予約" },
              ]}
              value={mode}
              onChange={setMode}
            />
            <p className="text-sm text-slate-500 dark:text-slate-400">
              {mode === "actual"
                ? "給与とは別の収入（副業・臨時収入など）を登録します。給与と合算して精算に反映されます。"
                : "入る予定はあるけれど金額がまだ決まっていない収入（賞与など）を登録しておきます。精算には含まれず、金額が確定したら一覧の「金額を入力」で収入として登録します。"}
            </p>
            <Field label="収入を得る人">
              <Select value={selectedMemberId} onChange={(e) => setMemberId(e.target.value)}>
                {members.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.name}
                  </option>
                ))}
              </Select>
            </Field>
            <div className="grid grid-cols-5 gap-3">
              <div className="col-span-3">
                <Field label="内容">
                  <Input
                    type="text" required
                    placeholder={mode === "actual" ? "副業など" : "賞与、還付金など"}
                    value={description} onChange={(e) => setDescription(e.target.value)}
                  />
                </Field>
              </div>
              <div className="col-span-2">
                <Field label={mode === "actual" ? "金額" : "見込み額"}>
                  <div className="relative">
                    <NumberInput
                      required={mode === "actual"}
                      placeholder={mode === "actual" ? "0" : "未定"}
                      value={amount} onChange={setAmount}
                      className="pr-8 text-right tabular-nums"
                    />
                    <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-slate-400">円</span>
                  </div>
                </Field>
              </div>
            </div>
            <Field label="頻度">
              <FrequencyToggle recurring={recurring} onChange={setRecurring} month={month} />
            </Field>
            <Button type="submit" disabled={busy} className="w-full">
              <PlusIcon className="h-5 w-5" />
              {busy ? "保存中..." : mode === "actual" ? "収入を追加" : "予約を追加"}
            </Button>
          </form>
        </Card>

        {/* 右列: 適用される収入（確定済み）と、収入の予約（未確定）を縦に並べる */}
        <div className="space-y-4 lg:col-span-3">
          <Card>
            <SectionTitle>{month} に適用される収入</SectionTitle>
            {incomes.loading ? (
              <Spinner />
            ) : list.length === 0 ? (
              <Empty>この月に適用される追加収入はありません</Empty>
            ) : (
              <ul className="divide-y divide-slate-100 dark:divide-slate-800">
                {list.map((inc) =>
                  editingId === inc.id && draft ? (
                    <li key={inc.id} className="py-3">
                      {/* インライン編集フォーム */}
                      <form
                        onSubmit={saveEdit}
                        className="space-y-3 rounded-xl bg-blue-50/70 p-3 ring-1 ring-blue-200 dark:bg-blue-950/30 dark:ring-blue-900"
                      >
                        <Field label="収入を得る人">
                          <Select
                            value={draft.memberId}
                            onChange={(ev) => setDraft((d) => (d ? { ...d, memberId: ev.target.value } : d))}
                          >
                            {members.map((m) => (
                              <option key={m.id} value={m.id}>
                                {m.name}
                              </option>
                            ))}
                          </Select>
                        </Field>
                        <div className="grid grid-cols-5 gap-3">
                          <div className="col-span-3">
                            <Field label="内容">
                              <Input
                                type="text" required placeholder="副業など"
                                value={draft.description}
                                onChange={(ev) => setDraft((d) => (d ? { ...d, description: ev.target.value } : d))}
                              />
                            </Field>
                          </div>
                          <div className="col-span-2">
                            <Field label="金額">
                              <div className="relative">
                                <NumberInput
                                  required placeholder="0"
                                  value={draft.amount}
                                  onChange={(v) => setDraft((d) => (d ? { ...d, amount: v } : d))}
                                  className="pr-8 text-right tabular-nums"
                                />
                                <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-slate-400">円</span>
                              </div>
                            </Field>
                          </div>
                        </div>
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
                    <li key={inc.id} className="flex items-center gap-2 py-3">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="truncate font-medium">{inc.description}</span>
                          <FrequencyBadge recurring={inc.recurring} />
                          {inc.reservationId && <ReservedBadge />}
                        </div>
                        <div className="mt-1">
                          <MemberBadge name={memberName(inc.memberId)} color={memberColor(inc.memberId)} />
                        </div>
                      </div>
                      <span className="whitespace-nowrap font-semibold tabular-nums">{yen(inc.amountYen)}</span>
                      <button
                        onClick={() => startEdit(inc)}
                        className="rounded-lg p-2 text-slate-400 hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-950/40"
                        aria-label="編集"
                      >
                        <EditIcon className="h-5 w-5" />
                      </button>
                      <button
                        onClick={() => remove(inc)}
                        className="rounded-lg p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-950/40"
                        aria-label="削除"
                      >
                        <TrashIcon className="h-5 w-5" />
                      </button>
                    </li>
                  )
                )}
              </ul>
            )}
          </Card>

          <ReservationList
            kind="income"
            month={month}
            members={members}
            notify={notify}
            onError={onError}
            onActualsChanged={incomes.reload}
            refreshKey={reservationVersion}
            closingDay={closingDay}
          />
        </div>
      </div>
    </div>
  );
}
