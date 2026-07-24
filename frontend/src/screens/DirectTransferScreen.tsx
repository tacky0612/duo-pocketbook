import { useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { yen } from "../lib/format";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Field, Input, NumberInput, Select, Button, Spinner, Empty, MemberBadge } from "../components/ui";
import { PlusIcon, TrashIcon, EditIcon, ArrowRightIcon } from "../components/Icons";
import type { DirectTransfer, DirectTransfersResponse, MemberId, ScreenProps } from "../types";

interface TransferDraft {
  from: MemberId;
  amount: string;
  description: string;
}

export default function DirectTransferScreen({ month, members, me, notify, onError }: ScreenProps) {
  const { loading, data, error, reload } = useAsync<DirectTransfersResponse>(
    () => api<DirectTransfersResponse>("GET", `/direct-transfers?month=${month}`),
    [month]
  );
  // 新規登録フォーム
  const [from, setFrom] = useState<MemberId>(me?.id || "");
  const [amount, setAmount] = useState("");
  const [description, setDescription] = useState("");
  const [recurring, setRecurring] = useState(true);
  // インライン編集
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState<TransferDraft | null>(null);
  const [busy, setBusy] = useState(false);

  if (error) onError(error);

  const memberName = (id: MemberId) => members.find((m) => m.id === id)?.name || id;
  const memberColor = (id: MemberId) => members.find((m) => m.id === id)?.color;
  // 送金元でない方（受け取る人）。
  const otherId = (id: MemberId) => members.find((m) => m.id !== id)?.id || id;

  const fromId = from || members[0]?.id || "";

  const add = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      await api("POST", "/direct-transfers", {
        from: fromId,
        amountYen: Number(amount),
        description,
        month: recurring ? "" : month,
      });
      setAmount("");
      setDescription("");
      notify("立替精算を登録しました");
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const startEdit = (t: DirectTransfer) => {
    setEditingId(t.id);
    setDraft({ from: t.from, amount: String(t.amountYen), description: t.description });
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
      await api("PUT", `/direct-transfers/${editingId}`, {
        from: draft.from,
        amountYen: Number(draft.amount),
        description: draft.description,
      });
      notify("立替精算を更新しました");
      cancelEdit();
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (t: DirectTransfer) => {
    if (!confirm(`立替精算「${t.description}」を削除しますか?`)) return;
    try {
      await api("DELETE", `/direct-transfers/${t.id}`);
      notify("立替精算を削除しました");
      if (editingId === t.id) cancelEdit();
      reload();
    } catch (err) {
      onError(err);
    }
  };

  const list = data?.directTransfers ?? [];

  // 送金元 → 送金先の名前を矢印付きで表示する小コンポーネント。
  const FromTo = ({ from, to }: { from: MemberId; to: MemberId }) => (
    <span className="inline-flex items-center gap-1.5">
      <MemberBadge name={memberName(from)} color={memberColor(from)} />
      <ArrowRightIcon className="h-4 w-4 text-slate-400" />
      <MemberBadge name={memberName(to)} color={memberColor(to)} />
    </span>
  );

  return (
    <div className="grid gap-4 lg:grid-cols-5 lg:items-start">
      <Card className="lg:col-span-2 lg:sticky lg:top-20">
        <SectionTitle>立替精算を追加</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          共有支出以外で一方が立て替えた分などを精算します。比重で按分されず、月次の振込額へそのまま加算されます。
        </p>
        <form onSubmit={add} className="space-y-4">
          <Field label="送る人 → 相手">
            <Select value={fromId} onChange={(e) => setFrom(e.target.value)}>
              {members.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.name} → {memberName(otherId(m.id))}
                </option>
              ))}
            </Select>
          </Field>
          <div className="grid grid-cols-5 gap-3">
            <div className="col-span-3">
              <Field label="内容">
                <Input type="text" required placeholder="立替の返済など" value={description} onChange={(e) => setDescription(e.target.value)} />
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
            {busy ? "保存中..." : "立替精算を追加"}
          </Button>
        </form>
      </Card>

      <Card className="lg:col-span-3">
        <SectionTitle>{month} に適用される立替精算</SectionTitle>
        {loading ? (
          <Spinner />
        ) : list.length === 0 ? (
          <Empty>この月に適用される立替精算はありません</Empty>
        ) : (
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {list.map((t) =>
              editingId === t.id && draft ? (
                <li key={t.id} className="py-3">
                  {/* インライン編集フォーム */}
                  <form
                    onSubmit={saveEdit}
                    className="space-y-3 rounded-xl bg-blue-50/70 p-3 ring-1 ring-blue-200 dark:bg-blue-950/30 dark:ring-blue-900"
                  >
                    <Field label="送る人 → 相手">
                      <Select
                        value={draft.from}
                        onChange={(ev) => setDraft((d) => (d ? { ...d, from: ev.target.value } : d))}
                      >
                        {members.map((m) => (
                          <option key={m.id} value={m.id}>
                            {m.name} → {memberName(otherId(m.id))}
                          </option>
                        ))}
                      </Select>
                    </Field>
                    <div className="grid grid-cols-5 gap-3">
                      <div className="col-span-3">
                        <Field label="内容">
                          <Input
                            type="text" required placeholder="立替の返済など"
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
                <li key={t.id} className="flex items-center gap-2 py-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate font-medium">{t.description}</span>
                      <span
                        className={
                          "shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium " +
                          (t.recurring
                            ? "bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300"
                            : "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400")
                        }
                      >
                        {t.recurring ? "毎月" : "今月だけ"}
                      </span>
                    </div>
                    <div className="mt-1">
                      <FromTo from={t.from} to={t.to} />
                    </div>
                  </div>
                  <span className="whitespace-nowrap font-semibold tabular-nums">{yen(t.amountYen)}</span>
                  <button
                    onClick={() => startEdit(t)}
                    className="rounded-lg p-2 text-slate-400 hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-950/40"
                    aria-label="編集"
                  >
                    <EditIcon className="h-5 w-5" />
                  </button>
                  <button
                    onClick={() => remove(t)}
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
