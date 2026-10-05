// 予約（金額未確定の収入・支出）の表示・入力で共通して使う部品群。
// 支出画面の「予約」タブ、収入画面の「収入の予約」、精算画面の未入力警告ダイアログで共有する。
import { useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { defaultDateInSettlementMonth } from "../lib/month";
import { Button, Field, Input, NumberInput } from "./ui";
import type { Notify, Reservation, ReservationKind, ReservationStatus, YearMonth } from "../types";

export const kindLabel: Record<ReservationKind, string> = { expense: "支出", income: "収入" };

// 予約のメンバー欄の意味（支出は支払う人、収入は収入を得る人）。
export const memberRoleLabel: Record<ReservationKind, string> = { expense: "支払う人", income: "収入を得る人" };

// 予約の「内容」欄の入力例。
export const reservationPlaceholder: Record<ReservationKind, string> = { expense: "電気代、車検など", income: "賞与、還付金など" };

const statusStyle: Record<ReservationStatus, { label: string; className: string }> = {
  pending: {
    label: "未入力",
    className: "bg-rose-100 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300",
  },
  fulfilled: {
    label: "入力済み",
    className: "bg-emerald-100 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300",
  },
  skipped: {
    label: "今月はなし",
    className: "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400",
  },
};

const pill = "shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium ";

// 入力状況のバッジ（未入力 / 入力済み / 今月はなし）。
export function StatusBadge({ status }: { status: ReservationStatus }) {
  const s = statusStyle[status];
  return <span className={pill + s.className}>{s.label}</span>;
}

// 頻度のバッジ（毎月 / 今月だけ）。既存の収入・立替精算の一覧と同じ配色にそろえる。
export function FrequencyBadge({ recurring }: { recurring: boolean }) {
  return (
    <span
      className={
        pill +
        (recurring
          ? "bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300"
          : "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400")
      }
    >
      {recurring ? "毎月" : "今月だけ"}
    </span>
  );
}

// 予約から登録された支出・収入であることを示すバッジ（支出一覧・収入一覧で使う）。
export function ReservedBadge() {
  return (
    <span className={pill + "bg-violet-100 text-violet-700 dark:bg-violet-950/50 dark:text-violet-300"}>予約</span>
  );
}

// 予約を対象月でスキップ（今月はなし）／解除する。
export function setReservationSkipped(r: Reservation, month: YearMonth, skipped: boolean): Promise<Reservation> {
  return api<Reservation>("PUT", `/reservations/${r.id}/skip`, { month, skipped });
}

interface FulfillReservationFormProps {
  reservation: Reservation;
  month: YearMonth;
  closingDay: number;
  notify: Notify;
  onError: (err: unknown) => void;
  onDone: () => void;
  onCancel?: () => void;
}

// 予約の金額入力フォーム。確定すると予約に紐づく支出（日付つき）／収入（対象月のみ）として登録される。
// 金額は見込み額があれば初期値に入れておく。支出の日付は今日（対象月外なら精算期間の初日）を初期値にする。
export function FulfillReservationForm({ reservation, month, closingDay, notify, onError, onDone, onCancel }: FulfillReservationFormProps) {
  const isExpense = reservation.kind === "expense";
  const [amount, setAmount] = useState(reservation.estimatedAmountYen > 0 ? String(reservation.estimatedAmountYen) : "");
  const [date, setDate] = useState(() => defaultDateInSettlementMonth(month, closingDay));
  const [busy, setBusy] = useState(false);

  const submit = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      await api("POST", `/reservations/${reservation.id}/fulfill`, {
        month,
        amountYen: Number(amount),
        date: isExpense ? date : "",
      });
      notify(`「${reservation.description}」を${kindLabel[reservation.kind]}として登録しました`);
      onDone();
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <form
      onSubmit={submit}
      className="mt-2 space-y-3 rounded-xl bg-blue-50/70 p-3 ring-1 ring-blue-200 dark:bg-blue-950/30 dark:ring-blue-900"
    >
      <div className={"grid gap-3 " + (isExpense ? "grid-cols-2" : "grid-cols-1")}>
        {isExpense && (
          <Field label="日付">
            {/* iOS Safari の date 入力はネイティブUIの固有幅で親をはみ出すため appearance-none で親幅に従わせる */}
            <Input type="date" required value={date} onChange={(e) => setDate(e.target.value)} className="min-w-0 appearance-none" />
          </Field>
        )}
        <Field label="確定した金額">
          <div className="relative">
            <NumberInput
              required placeholder="0" autoFocus
              value={amount} onChange={setAmount}
              className="pr-8 text-right tabular-nums"
            />
            <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-slate-400">円</span>
          </div>
        </Field>
      </div>
      <div className="flex gap-2">
        {onCancel && (
          <Button type="button" variant="secondary" onClick={onCancel} className="flex-1">
            キャンセル
          </Button>
        )}
        <Button type="submit" disabled={busy} className="flex-1">
          {busy ? "登録中..." : `${kindLabel[reservation.kind]}として登録`}
        </Button>
      </div>
    </form>
  );
}
