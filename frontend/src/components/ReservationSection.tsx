import { useState, type FormEvent } from "react";
import { api } from "../lib/apiClient";
import { Card, SectionTitle, Field, Input, Select, Button } from "./ui";
import { PlusIcon } from "./Icons";
import FrequencyToggle from "./FrequencyToggle";
import ReservationList from "./ReservationList";
import { kindLabel, memberRoleLabel, reservationPlaceholder } from "./ReservationParts";
import type { Member, MemberId, MemberView, Notify, ReservationKind, YearMonth } from "../types";

interface ReservationSectionProps {
  kind: ReservationKind;
  month: YearMonth;
  members: MemberView[];
  me: Member;
  notify: Notify;
  onError: (err: unknown) => void;
  closingDay: number;
  // 金額入力フォームを開いた状態で表示する予約のID（精算画面から遷移した場合）。
  fulfillReservationId?: string;
}

// 予約の登録フォーム＋対象月の予約一覧。支出画面の「予約」タブで使う。
// （収入画面は通常の収入と予約を同じフォームで登録するため、一覧の ReservationList だけを使う）
export default function ReservationSection({ kind, month, members, me, notify, onError, closingDay, fulfillReservationId }: ReservationSectionProps) {
  const [memberId, setMemberId] = useState<MemberId>(me?.id || "");
  const [description, setDescription] = useState("");
  // 支出の予約は「今月だけ発生する予定」（車検など）が多いため、対象月のみを初期値にする。
  const [recurring, setRecurring] = useState(false);
  const [busy, setBusy] = useState(false);
  // 登録のたびに一覧を再取得させるためのカウンタ。
  const [version, setVersion] = useState(0);

  const selectedMemberId = memberId || members[0]?.id || "";
  const label = kindLabel[kind];

  const add = async (ev: FormEvent<HTMLFormElement>) => {
    ev.preventDefault();
    setBusy(true);
    try {
      await api("POST", "/reservations", {
        kind,
        memberId: selectedMemberId,
        description,
        month: recurring ? "" : month,
        // 毎月の予約は表示中の月から始める（登録より前の月に未入力として現れないように）。
        startMonth: recurring ? month : "",
      });
      setDescription("");
      notify(`${label}の予約を登録しました`);
      setVersion((v) => v + 1);
    } catch (err) {
      onError(err);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="grid gap-4 lg:grid-cols-5 lg:items-start">
      <Card className="lg:col-span-2 lg:sticky lg:top-20">
        <SectionTitle>{label}の予約を追加</SectionTitle>
        <p className="mb-4 text-sm text-slate-500 dark:text-slate-400">
          発生する予定はあるけれど金額がまだ決まっていない{label}を登録しておきます。金額が確定したら「金額を入力」で{label}として登録します。
        </p>
        <form onSubmit={add} className="space-y-4">
          <Field label={memberRoleLabel[kind]}>
            <Select value={selectedMemberId} onChange={(e) => setMemberId(e.target.value)}>
              {members.map((m) => (
                <option key={m.id} value={m.id}>{m.name}</option>
              ))}
            </Select>
          </Field>
          <Field label="内容">
            <Input type="text" required placeholder={reservationPlaceholder[kind]} value={description} onChange={(e) => setDescription(e.target.value)} />
          </Field>
          <Field label="頻度">
            <FrequencyToggle recurring={recurring} onChange={setRecurring} month={month} />
          </Field>
          <Button type="submit" disabled={busy} className="w-full">
            <PlusIcon className="h-5 w-5" />
            {busy ? "保存中..." : "予約を追加"}
          </Button>
        </form>
      </Card>

      <div className="lg:col-span-3">
        <ReservationList kind={kind} month={month} members={members} notify={notify} onError={onError} refreshKey={version} closingDay={closingDay} initialFulfillingId={fulfillReservationId} />
      </div>
    </div>
  );
}
