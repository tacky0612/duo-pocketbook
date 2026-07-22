import { useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { yen } from "../lib/format";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Field, Input, NumberInput, Select, Button, Spinner, Empty, MemberBadge } from "../components/ui";
import { PlusIcon, TrashIcon, EditIcon } from "../components/Icons";
import type { MemberId, RecurringExpense, RecurringExpensesResponse, ScreenProps } from "../types";

interface RecurringDraft {
  paidBy: MemberId;
  amount: string;
  description: string;
}

export default function RecurringScreen({ members, me, notify, onError }: ScreenProps) {
  const { loading, data, error, reload } = useAsync<RecurringExpensesResponse>(
    () => api<RecurringExpensesResponse>("GET", "/recurring-expenses"),
    []
  );
  // 新規登録フォーム
  const [paidBy, setPaidBy] = useState<MemberId>(me?.id || "");
  const [amount, setAmount] = useState("");
  const [description, setDescription] = useState("");
  // インライン編集
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<RecurringDraft | null>(null);
  const [busy, setBusy] = useState(false);

  if (error) onError(error);

  const memberName = (id: MemberId) => members.find((m) => m.id === id)?.name || id;
  const memberColor = (id: MemberId) => members.find((m) => m.id === id)?.color;

  const add = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      await api("POST", "/recurring-expenses", {
        paidBy: paidBy || members[0]?.id,
        amountYen: Number(amount),
        description,
      });
      setAmount("");
      setDescription("");
      notify("固定費を登録しました");
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const startEdit = (e: RecurringExpense) => {
    setEditingId(e.id);
    setDraft({ paidBy: e.paidBy, amount: String(e.amountYen), description: e.description });
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
      await api("PUT", `/recurring-expenses/${editingId}`, {
        paidBy: draft.paidBy,
        amountYen: Number(draft.amount),
        description: draft.description,
      });
      notify("固定費を更新しました");
      cancelEdit();
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (e: RecurringExpense) => {
    if (!confirm(`固定費「${e.description}」を削除しますか?`)) return;
    try {
      await api("DELETE", `/recurring-expenses/${e.id}`);
      notify("固定費を削除しました");
      if (editingId === e.id) cancelEdit();
      reload();
    } catch (err) {
      onError(err);
    }
  };

  const total = data?.recurringExpenses.reduce((s, e) => s + e.amountYen, 0) ?? 0;

  return (
    <div className="grid gap-4 lg:grid-cols-5 lg:items-start">
      <Card className="lg:col-span-2 lg:sticky lg:top-20">
        <SectionTitle>固定費を追加</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          家賃・光熱費・サブスクなど、毎月発生する共有支出です。すべての月の精算に自動で含まれます。
        </p>
        <form onSubmit={add} className="space-y-4">
          <Field label="毎月支払う人">
            <Select value={paidBy || members[0]?.id || ""} onChange={(e) => setPaidBy(e.target.value)}>
              {members.map((m) => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </Select>
          </Field>
          <div className="grid grid-cols-5 gap-3">
            <div className="col-span-3">
              <Field label="内容">
                <Input type="text" required placeholder="家賃など" value={description} onChange={(e) => setDescription(e.target.value)} />
              </Field>
            </div>
            <div className="col-span-2">
              <Field label="金額（月額）">
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
            {busy ? "保存中..." : "固定費を追加"}
          </Button>
        </form>
      </Card>

      <Card className="lg:col-span-3">
        <SectionTitle
          action={
            data?.recurringExpenses.length ? (
              <span className="text-sm text-slate-400">
                月額計 <span className="font-semibold tabular-nums text-slate-600 dark:text-slate-300">{yen(total)}</span>
              </span>
            ) : null
          }
        >
          登録済みの固定費
        </SectionTitle>
        {loading ? (
          <Spinner />
        ) : !data?.recurringExpenses.length ? (
          <Empty>固定費はまだ登録されていません</Empty>
        ) : (
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {data.recurringExpenses.map((e) =>
              editingId === e.id && draft ? (
                <li key={e.id} className="py-3">
                  {/* インライン編集フォーム */}
                  <form
                    onSubmit={saveEdit}
                    className="space-y-3 rounded-xl bg-blue-50/70 p-3 ring-1 ring-blue-200 dark:bg-blue-950/30 dark:ring-blue-900"
                  >
                    <Field label="毎月支払う人">
                      <Select
                        value={draft.paidBy}
                        onChange={(ev) => setDraft((d) => (d ? { ...d, paidBy: ev.target.value } : d))}
                      >
                        {members.map((m) => (
                          <option key={m.id} value={m.id}>{m.name}</option>
                        ))}
                      </Select>
                    </Field>
                    <div className="grid grid-cols-5 gap-3">
                      <div className="col-span-3">
                        <Field label="内容">
                          <Input
                            type="text" required placeholder="家賃など"
                            value={draft.description}
                            onChange={(ev) => setDraft((d) => (d ? { ...d, description: ev.target.value } : d))}
                          />
                        </Field>
                      </div>
                      <div className="col-span-2">
                        <Field label="金額（月額）">
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
                    <div className="truncate font-medium">{e.description}</div>
                    <div className="mt-0.5">
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
