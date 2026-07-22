import { useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { yen } from "../lib/format";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Field, Input, NumberInput, Select, Button, Spinner, Empty, MemberBadge } from "../components/ui";
import { PlusIcon, TrashIcon, EditIcon } from "../components/Icons";
import type { Expense, ExpensesResponse, MemberId, ScreenProps } from "../types";

interface ExpenseDraft {
  paidBy: MemberId;
  amount: string;
  description: string;
  date: string;
}

function todayIn(month: string): string {
  const [y, m] = month.split("-").map(Number);
  const now = new Date();
  // 表示中の月なら今日、そうでなければ月初をデフォルトにする
  if (now.getFullYear() === y && now.getMonth() + 1 === m) {
    return `${y}-${String(m).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
  }
  return `${month}-01`;
}

export default function ExpenseScreen({ month, members, me, notify, onError }: ScreenProps) {
  const { loading, data, error, reload } = useAsync<ExpensesResponse>(
    () => api<ExpensesResponse>("GET", `/expenses?month=${month}`),
    [month]
  );
  // 新規登録フォーム
  const [paidBy, setPaidBy] = useState<MemberId>(me?.id || "");
  const [amount, setAmount] = useState("");
  const [description, setDescription] = useState("");
  const [date, setDate] = useState(todayIn(month));
  // インライン編集
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<ExpenseDraft | null>(null);
  const [busy, setBusy] = useState(false);

  if (error) onError(error);

  const memberName = (id: MemberId) => members.find((m) => m.id === id)?.name || id;
  const memberColor = (id: MemberId) => members.find((m) => m.id === id)?.color;

  const add = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      await api("POST", "/expenses", {
        paidBy: paidBy || members[0]?.id,
        amountYen: Number(amount),
        description,
        date,
      });
      setAmount("");
      setDescription("");
      notify("支出を登録しました");
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const startEdit = (e: Expense) => {
    setEditingId(e.id);
    setDraft({ paidBy: e.paidBy, amount: String(e.amountYen), description: e.description, date: e.date });
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
      await api("PUT", `/expenses/${editingId}`, {
        paidBy: draft.paidBy,
        amountYen: Number(draft.amount),
        description: draft.description,
        date: draft.date,
      });
      notify("支出を更新しました");
      cancelEdit();
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (e: Expense) => {
    if (!confirm(`「${e.description || yen(e.amountYen)}」を削除しますか?`)) return;
    try {
      await api("DELETE", `/expenses/${e.id}`);
      notify("支出を削除しました");
      if (editingId === e.id) cancelEdit();
      reload();
    } catch (err) {
      onError(err);
    }
  };

  const total = data?.expenses.reduce((s, e) => s + e.amountYen, 0) ?? 0;

  return (
    <div className="grid gap-4 lg:grid-cols-5 lg:items-start">
      <Card className="lg:col-span-2 lg:sticky lg:top-20">
        <SectionTitle>支出を登録</SectionTitle>
        <form onSubmit={add} className="space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <Field label="支払った人">
              <Select value={paidBy || members[0]?.id || ""} onChange={(e) => setPaidBy(e.target.value)}>
                {members.map((m) => (
                  <option key={m.id} value={m.id}>{m.name}</option>
                ))}
              </Select>
            </Field>
            <Field label="日付">
              <Input type="date" required value={date} onChange={(e) => setDate(e.target.value)} />
            </Field>
          </div>
          <div className="grid grid-cols-5 gap-3">
            <div className="col-span-3">
              <Field label="内容">
                <Input type="text" placeholder="食費、日用品など" value={description} onChange={(e) => setDescription(e.target.value)} />
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
          <Button type="submit" disabled={busy} className="w-full">
            <PlusIcon className="h-5 w-5" />
            {busy ? "保存中..." : "登録する"}
          </Button>
        </form>
      </Card>

      <Card className="lg:col-span-3">
        <SectionTitle
          action={
            data?.expenses.length ? (
              <span className="text-sm text-slate-400">
                合計 <span className="font-semibold tabular-nums text-slate-600 dark:text-slate-300">{yen(total)}</span>
              </span>
            ) : null
          }
        >
          支出一覧
        </SectionTitle>
        {loading ? (
          <Spinner />
        ) : !data?.expenses.length ? (
          <Empty>この月の支出はまだありません</Empty>
        ) : (
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {data.expenses.map((e) =>
              editingId === e.id && draft ? (
                <li key={e.id} className="py-3">
                  {/* インライン編集フォーム */}
                  <form
                    onSubmit={saveEdit}
                    className="space-y-3 rounded-xl bg-blue-50/70 p-3 ring-1 ring-blue-200 dark:bg-blue-950/30 dark:ring-blue-900"
                  >
                    <div className="grid grid-cols-2 gap-3">
                      <Field label="支払った人">
                        <Select
                          value={draft.paidBy}
                          onChange={(ev) => setDraft((d) => (d ? { ...d, paidBy: ev.target.value } : d))}
                        >
                          {members.map((m) => (
                            <option key={m.id} value={m.id}>{m.name}</option>
                          ))}
                        </Select>
                      </Field>
                      <Field label="日付">
                        <Input
                          type="date" required
                          value={draft.date}
                          onChange={(ev) => setDraft((d) => (d ? { ...d, date: ev.target.value } : d))}
                        />
                      </Field>
                    </div>
                    <div className="grid grid-cols-5 gap-3">
                      <div className="col-span-3">
                        <Field label="内容">
                          <Input
                            type="text" placeholder="食費、日用品など"
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
                <li key={e.id} className="flex items-center gap-2 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="truncate font-medium">{e.description || "（内容なし）"}</div>
                    <div className="mt-0.5 flex items-center gap-2 text-xs text-slate-400">
                      <span className="tabular-nums">{e.date}</span>
                      <MemberBadge name={memberName(e.paidBy)} color={memberColor(e.paidBy)} />
                    </div>
                  </div>
                  <span className="whitespace-nowrap font-semibold tabular-nums">{yen(e.amountYen)}</span>
                  <button
                    onClick={() => startEdit(e)}
                    className="rounded-lg p-2 text-slate-400 hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-950/40"
                    aria-label="編集"
                  >
                    <EditIcon className="h-5 w-5" />
                  </button>
                  <button
                    onClick={() => remove(e)}
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
  );
}
