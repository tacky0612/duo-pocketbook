import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { useAsync } from "../hooks";
import { Card, SectionTitle, Field, NumberInput, Button, Spinner } from "../components/ui";
import type { IncomesResponse, ScreenProps } from "../types";

export default function IncomeScreen({ month, members, notify, onError }: ScreenProps) {
  const { loading, data, error, reload } = useAsync<IncomesResponse>(
    () => api<IncomesResponse>("GET", `/months/${month}/incomes`),
    [month]
  );
  const [values, setValues] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!data) return;
    const next: Record<string, string> = {};
    for (const m of members) {
      const inc = data.incomes.find((i) => i.memberId === m.id);
      next[m.id] = inc != null ? String(inc.amountYen) : "";
    }
    setValues(next);
  }, [data, members]);

  if (error) onError(error);
  if (loading) return <Spinner />;

  const submit = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      for (const m of members) {
        const v = values[m.id];
        if (v !== "" && v != null) {
          await api("PUT", `/months/${month}/incomes/${m.id}`, { amountYen: Number(v) });
        }
      }
      notify("収入を保存しました");
      reload();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto w-full max-w-3xl">
      <Card>
        <SectionTitle>月次の収入</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          この月のふたりの収入（手取り）を入力してください。精算額の計算に使われます。
        </p>
        <form onSubmit={submit} className="space-y-4">
          <div className="grid gap-4 sm:grid-cols-2">
            {members.map((m) => (
              <Field key={m.id} label={`${m.name} の収入`}>
                <div className="relative">
                  <NumberInput
                    placeholder="0"
                    value={values[m.id] ?? ""}
                    onChange={(v) => setValues((p) => ({ ...p, [m.id]: v }))}
                    className="pr-10 text-right tabular-nums"
                  />
                  <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-slate-400">
                    円
                  </span>
                </div>
              </Field>
            ))}
          </div>
          <Button type="submit" disabled={busy} className="w-full sm:w-auto">
            {busy ? "保存中..." : "収入を保存"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
