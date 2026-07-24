import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { yen } from "../lib/format";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Field, Input, NumberInput, Select, Button, Spinner, Empty, MemberBadge } from "../components/ui";
import { PlusIcon, TrashIcon, EditIcon } from "../components/Icons";
import type { Income, IncomesResponse, MemberId, SalariesResponse, ScreenProps } from "../types";

interface IncomeDraft {
  memberId: MemberId;
  amount: string;
  description: string;
}

export default function IncomeScreen({ month, members, me, notify, onError }: ScreenProps) {
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

  const add = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      await api("POST", "/incomes", {
        memberId: selectedMemberId,
        amountYen: Number(amount),
        description,
        month: recurring ? "" : month,
      });
      setAmount("");
      setDescription("");
      notify("収入を登録しました");
      incomes.reload();
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
          <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
            給与とは別の収入（副業・臨時収入など）を登録します。給与と合算して精算に反映されます。
          </p>
          <form onSubmit={add} className="space-y-4">
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
                  <Input type="text" required placeholder="副業など" value={description} onChange={(e) => setDescription(e.target.value)} />
                </Field>
              </div>
              <div className="col-span-2">
                <Field label="金額">
                  <div className="relative">
                    <NumberInput
                      required placeholder="0"
                      value={amount} onChange={setAmount}
                      className="pr-8 text-right tabular-nums"
                    />
                    <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-slate-400">円</span>
                  </div>
                </Field>
              </div>
            </div>
            <Field label="頻度">
              <div className="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  onClick={() => setRecurring(true)}
                  className={
                    "rounded-xl border px-3 py-2 text-sm font-medium transition-colors " +
                    (recurring
                      ? "border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-500 dark:bg-blue-950/40 dark:text-blue-300"
                      : "border-slate-200 text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800")
                  }
                >
                  毎月継続
                </button>
                <button
                  type="button"
                  onClick={() => setRecurring(false)}
                  className={
                    "rounded-xl border px-3 py-2 text-sm font-medium transition-colors " +
                    (!recurring
                      ? "border-blue-500 bg-blue-50 text-blue-700 dark:border-blue-500 dark:bg-blue-950/40 dark:text-blue-300"
                      : "border-slate-200 text-slate-500 hover:bg-slate-50 dark:border-slate-700 dark:hover:bg-slate-800")
                  }
                >
                  {month} のみ
                </button>
              </div>
            </Field>
            <Button type="submit" disabled={busy} className="w-full">
              <PlusIcon className="h-5 w-5" />
              {busy ? "保存中..." : "収入を追加"}
            </Button>
          </form>
        </Card>

        <Card className="lg:col-span-3">
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
                        <span
                          className={
                            "shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium " +
                            (inc.recurring
                              ? "bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300"
                              : "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400")
                          }
                        >
                          {inc.recurring ? "毎月" : "今月だけ"}
                        </span>
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
      </div>
    </div>
  );
}
