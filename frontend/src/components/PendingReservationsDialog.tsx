import { useEffect, useState } from "react";
import { yen } from "../lib/format";
import { AlertIcon, CheckIcon } from "./Icons";
import { Button, MemberBadge, Modal } from "./ui";
import { FrequencyBadge, FulfillReservationForm, kindLabel, setReservationSkipped } from "./ReservationParts";
import type { MemberId, MemberView, Notify, Reservation, YearMonth } from "../types";

interface PendingReservationsDialogProps {
  open: boolean;
  month: YearMonth;
  // 対象月の未入力（status=pending）の予約。
  pending: Reservation[];
  // 予約の取得に失敗したときのメッセージ。未入力の有無を確認できないことを伝える。
  loadError?: string | null;
  members: MemberView[];
  closingDay: number;
  busy: boolean;
  notify: Notify;
  onError: (err: unknown) => void;
  onClose: () => void;
  // 金額入力・スキップで状況が変わったときに呼ぶ（精算画面を再取得する）。
  onChanged: () => void;
  // 精算を完了する（未入力が残っていてもこのまま完了する）。
  onConfirm: () => void;
}

// 精算確定時に、対象月に未入力の予約が残っていることを警告するダイアログ。
// その場で金額を入力するか「今月はなし」にでき、未入力のまま完了することもできる。
export default function PendingReservationsDialog({
  open, month, pending, loadError, members, closingDay, busy, notify, onError, onClose, onChanged, onConfirm,
}: PendingReservationsDialogProps) {
  const [fulfillingId, setFulfillingId] = useState<string | null>(null);
  const [skippingId, setSkippingId] = useState<string | null>(null);

  // 閉じたら開いていた金額入力フォームを畳む（次に開いたとき途中の入力が残らないように）。
  // 精算の完了で親が直接閉じる場合もあるため、open の変化で判定する。
  useEffect(() => {
    if (!open) setFulfillingId(null);
  }, [open]);

  const memberName = (id: MemberId) => members.find((m) => m.id === id)?.name || id;
  const memberColor = (id: MemberId) => members.find((m) => m.id === id)?.color;

  const skip = async (r: Reservation) => {
    if (skippingId) return; // 連打による二重送信を防ぐ
    setSkippingId(r.id);
    try {
      await setReservationSkipped(r, month, true);
      notify(`「${r.description}」を今月はなしにしました`);
      onChanged();
    } catch (err) {
      onError(err);
    } finally {
      setSkippingId(null);
    }
  };

  const remaining = pending.length;
  // 未入力が残っている、または予約を確認できなかった場合は警告として扱う。
  const warn = remaining > 0 || Boolean(loadError);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={
        loadError ? (
          <span className="flex items-center gap-2">
            <AlertIcon className="h-5 w-5 text-amber-500" />
            予約を確認できませんでした
          </span>
        ) : remaining > 0 ? (
          <span className="flex items-center gap-2">
            <AlertIcon className="h-5 w-5 text-amber-500" />
            未入力の予約があります
          </span>
        ) : (
          <span className="flex items-center gap-2">
            <CheckIcon className="h-5 w-5 text-emerald-500" />
            予約はすべて入力済みです
          </span>
        )
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            キャンセル
          </Button>
          <Button
            onClick={onConfirm}
            disabled={busy}
            className={warn ? "!bg-amber-600 hover:!bg-amber-500 !shadow-amber-600/20" : ""}
          >
            <CheckIcon className="h-5 w-5" />
            {busy ? "処理中..." : loadError ? "確認せずに完了する" : remaining > 0 ? "未入力のまま完了する" : "精算を完了する"}
          </Button>
        </>
      }
    >
      {loadError ? (
        <p className="text-sm text-slate-600 dark:text-slate-300">
          予約の取得に失敗したため、{month} に未入力の予約があるかを確認できませんでした（{loadError}）。
          時間をおいて画面を開き直すか、このまま完了してください。
        </p>
      ) : remaining > 0 ? (
        <>
          <p className="mb-3 text-sm text-slate-600 dark:text-slate-300">
            {month} に金額が未入力の予約が <span className="font-semibold">{remaining}件</span> あります。
            このまま完了すると、これらは精算に含まれません。
          </p>
          <ul className="divide-y divide-slate-100 dark:divide-slate-800">
            {pending.map((r) => (
              <li key={r.id} className="py-3">
                <div className="flex items-center gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="shrink-0 rounded-full bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-950/60 dark:text-blue-300">
                        {kindLabel[r.kind]}
                      </span>
                      <span className="truncate font-medium">{r.description}</span>
                      <FrequencyBadge recurring={r.recurring} />
                    </div>
                    <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-slate-400">
                      <MemberBadge name={memberName(r.memberId)} color={memberColor(r.memberId)} />
                      <span className="tabular-nums">見込み {r.estimatedAmountYen > 0 ? yen(r.estimatedAmountYen) : "未定"}</span>
                    </div>
                  </div>
                </div>
                {fulfillingId === r.id ? (
                  <FulfillReservationForm
                    reservation={r}
                    month={month}
                    closingDay={closingDay}
                    notify={notify}
                    onError={onError}
                    onDone={() => {
                      setFulfillingId(null);
                      onChanged();
                    }}
                    onCancel={() => setFulfillingId(null)}
                  />
                ) : (
                  <div className="mt-2 flex flex-wrap gap-2">
                    <Button onClick={() => setFulfillingId(r.id)} className="px-3 py-1.5 text-xs">
                      金額を入力
                    </Button>
                    {/* 「今月はなし」は毎月の予約のみ */}
                    {r.recurring && (
                      <Button variant="secondary" onClick={() => skip(r)} disabled={skippingId !== null} className="px-3 py-1.5 text-xs">
                        今月はなし
                      </Button>
                    )}
                  </div>
                )}
              </li>
            ))}
          </ul>
        </>
      ) : (
        <p className="text-sm text-slate-600 dark:text-slate-300">
          {month} の予約はすべて入力済み（または今月はなし）です。精算を完了できます。
        </p>
      )}
    </Modal>
  );
}
