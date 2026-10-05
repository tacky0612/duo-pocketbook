package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
	"github.com/tacky0612/duo-pocketbook/internal/infrastructure/memory"
)

func statusOf(t *testing.T, f fixture, month string, id domain.ReservationID) domain.ReservationState {
	t.Helper()
	states, err := f.reserve.ListForMonth(context.Background(), month, "")
	if err != nil {
		t.Fatalf("ListForMonth: %v", err)
	}
	for _, s := range states {
		if s.Reservation.ID == id {
			return s
		}
	}
	t.Fatalf("予約 %s が %s の一覧にありません", id, month)
	return domain.ReservationState{}
}

func TestReservationExpenseFulfillFlow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	r, err := f.reserve.Register(ctx, application.RegisterReservationInput{
		Kind: "expense", MemberID: husband, Description: "電気代", EstimatedAmountYen: 8000,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !r.IsRecurring() {
		t.Fatalf("month 未指定は毎月の予約になるべき: %+v", r)
	}
	// 毎月の予約は各月に未入力として現れる。
	for _, m := range []string{"2026-07", "2026-08"} {
		if s := statusOf(t, f, m, r.ID); s.Status != domain.ReservationPending {
			t.Errorf("%s: status = %v, want pending", m, s.Status)
		}
	}

	// 精算期間外の日付は拒否する。
	if _, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 7820, Date: "2026-08-01"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("期間外の日付: err = %v, want ErrValidation", err)
	}

	state, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 7820, Date: "2026-07-25"})
	if err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
	if state.Status != domain.ReservationFulfilled || state.FulfilledAmount != 7820 {
		t.Fatalf("state = %+v", state)
	}
	// 予約IDが紐づいた共有支出として登録され、精算に反映される。
	expenses, err := f.expenses.ListByMonth(ctx, "2026-07")
	if err != nil || len(expenses) != 1 {
		t.Fatalf("ListByMonth = %v, %v", expenses, err)
	}
	e := expenses[0]
	if e.Reservation != domain.NewReservationRef(r.ID) || e.Description != "電気代" || e.PaidBy != husband || string(e.ID) != state.FulfilledIDs[0] {
		t.Errorf("expense = %+v", e)
	}
	// 他の月は未入力のまま。
	if s := statusOf(t, f, "2026-08", r.ID); s.Status != domain.ReservationPending {
		t.Errorf("2026-08: status = %v, want pending", s.Status)
	}
	// 同じ月への二重入力は拒否する。
	if _, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 1, Date: "2026-07-26"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("二重入力: err = %v, want ErrValidation", err)
	}

	// 支出を編集しても予約との紐づけは維持される。
	if _, err := f.expenses.Update(ctx, e.ID, application.RegisterExpenseInput{PaidBy: wife, AmountYen: 9000, Description: "電気代（10月分）", Date: "2026-07-20"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// 予約から登録した支出は別の精算月へ移せない。
	if _, err := f.expenses.Update(ctx, e.ID, application.RegisterExpenseInput{PaidBy: wife, AmountYen: 9000, Description: "電気代", Date: "2026-08-03"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("別の月へ移す編集: err = %v, want ErrValidation", err)
	}
	if s := statusOf(t, f, "2026-07", r.ID); s.Status != domain.ReservationFulfilled || s.FulfilledAmount != 9000 {
		t.Errorf("編集後: %+v", s)
	}
	// 支出を削除すると未入力に戻る。
	if err := f.expenses.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s := statusOf(t, f, "2026-07", r.ID); s.Status != domain.ReservationPending {
		t.Errorf("削除後: status = %v, want pending", s.Status)
	}
}

func TestReservationIncomeFulfill(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	r, err := f.reserve.Register(ctx, application.RegisterReservationInput{
		Kind: "income", MemberID: wife, Description: "賞与", Month: "2026-07",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if r.IsRecurring() || r.EstimatedAmount != 0 {
		t.Fatalf("reservation = %+v", r)
	}
	// 単発の予約は対象月だけに現れる。
	if list, _ := f.reserve.ListForMonth(ctx, "2026-08", ""); len(list) != 0 {
		t.Errorf("2026-08 に単発予約が現れた: %+v", list)
	}
	// 収入の予約は日付不要。
	if _, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 300000}); err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
	incomes, err := f.income.ListForMonth(ctx, "2026-07")
	if err != nil || len(incomes) != 1 {
		t.Fatalf("ListForMonth = %v, %v", incomes, err)
	}
	if inc := incomes[0]; inc.Reservation != domain.NewReservationRef(r.ID) || inc.IsRecurring() || inc.Amount != 300000 || inc.MemberID != wife {
		t.Errorf("income = %+v", inc)
	}
	// 種別で絞り込める。
	if list, _ := f.reserve.ListForMonth(ctx, "2026-07", "expense"); len(list) != 0 {
		t.Errorf("kind=expense に収入予約が含まれた: %+v", list)
	}
	if _, err := f.reserve.ListForMonth(ctx, "2026-07", "unknown"); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("不明な kind: err = %v", err)
	}
	// 予約がない月への入力は拒否する。
	if _, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-08", AmountYen: 1}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("対象外の月: err = %v, want ErrValidation", err)
	}
}

func TestReservationSkip(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	r, err := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "医療費"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	s, err := f.reserve.SetSkipped(ctx, r.ID, "2026-07", true)
	if err != nil || s.Status != domain.ReservationSkipped {
		t.Fatalf("SetSkipped = %+v, %v", s, err)
	}
	if s := statusOf(t, f, "2026-08", r.ID); s.Status != domain.ReservationPending {
		t.Errorf("スキップは月ごと: 2026-08 = %v", s.Status)
	}
	if s, err := f.reserve.SetSkipped(ctx, r.ID, "2026-07", false); err != nil || s.Status != domain.ReservationPending {
		t.Errorf("解除 = %+v, %v", s, err)
	}

	// スキップ中に入力するとスキップは解除され、入力済みはスキップできない。
	if _, err := f.reserve.SetSkipped(ctx, r.ID, "2026-07", true); err != nil {
		t.Fatalf("SetSkipped: %v", err)
	}
	state, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 3000, Date: "2026-07-03"})
	if err != nil || state.Status != domain.ReservationFulfilled {
		t.Fatalf("Fulfill = %+v, %v", state, err)
	}
	if _, err := f.reserve.SetSkipped(ctx, r.ID, "2026-07", true); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("入力済みのスキップ: err = %v, want ErrValidation", err)
	}
	// 支出を消しても、入力時に解除したスキップは復活しない。
	if err := f.expenses.Delete(ctx, domain.ExpenseID(state.FulfilledIDs[0])); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s := statusOf(t, f, "2026-07", r.ID); s.Status != domain.ReservationPending {
		t.Errorf("status = %v, want pending", s.Status)
	}
}

func TestReservationUpdateAndDelete(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	r, err := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "車検", Month: "2026-07"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// 種別は更新で変わらない。頻度・対象月が同じならIDも変わらない。
	u, err := f.reserve.Update(ctx, r.ID, application.RegisterReservationInput{Kind: "income", MemberID: wife, Description: "車検代", EstimatedAmountYen: 60000, Month: "2026-07"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if u.ID != r.ID || u.Kind != domain.ReservationKindExpense || u.Month != r.Month || u.MemberID != wife || u.EstimatedAmount != 60000 || u.Description != "車検代" {
		t.Errorf("updated = %+v", u)
	}
	if _, err := f.reserve.Update(ctx, "rsv_missing", application.RegisterReservationInput{MemberID: wife, Description: "x"}); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("存在しない予約: err = %v", err)
	}
	if err := f.reserve.Delete(ctx, r.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if list, _ := f.reserve.ListForMonth(ctx, "2026-07", ""); len(list) != 0 {
		t.Errorf("削除後も残っている: %+v", list)
	}
}

func TestReservationValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	cases := []application.RegisterReservationInput{
		{Kind: "transfer", MemberID: husband, Description: "x"},
		{Kind: "expense", MemberID: "unknown", Description: "x"},
		{Kind: "expense", MemberID: husband, Description: ""},
		{Kind: "expense", MemberID: husband, Description: "x", EstimatedAmountYen: -1},
		{Kind: "expense", MemberID: husband, Description: "x", Month: "2026/07"},
	}
	for _, in := range cases {
		if _, err := f.reserve.Register(ctx, in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("Register(%+v): err = %v, want ErrValidation", in, err)
		}
	}
	r, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "x"})
	if _, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 0, Date: "2026-07-01"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("金額0: err = %v, want ErrValidation", err)
	}
	if _, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 100}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("支出の予約で日付なし: err = %v, want ErrValidation", err)
	}
}

func TestReservationSettledMonthLocked(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	recurring, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "電気代"})
	oneOff, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "income", MemberID: wife, Description: "賞与", Month: "2026-07"})
	for _, m := range []domain.MemberID{husband, wife} {
		if _, err := f.settlement.InputSalary(ctx, "2026-07", m, 100000); err != nil {
			t.Fatalf("InputSalary: %v", err)
		}
	}
	// 未入力の予約があっても精算の確定自体はできる（警告は UI が出す）。
	if _, err := f.settlement.SetSettled(ctx, "2026-07", true); err != nil {
		t.Fatalf("SetSettled: %v", err)
	}

	if _, err := f.reserve.Fulfill(ctx, recurring.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 1, Date: "2026-07-01"}); !errors.Is(err, domain.ErrSettled) {
		t.Errorf("確定済み月への入力: err = %v, want ErrSettled", err)
	}
	if _, err := f.reserve.SetSkipped(ctx, recurring.ID, "2026-07", true); !errors.Is(err, domain.ErrSettled) {
		t.Errorf("確定済み月のスキップ: err = %v, want ErrSettled", err)
	}
	if _, err := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "x", Month: "2026-07"}); !errors.Is(err, domain.ErrSettled) {
		t.Errorf("確定済み月への単発登録: err = %v, want ErrSettled", err)
	}
	if err := f.reserve.Delete(ctx, oneOff.ID); !errors.Is(err, domain.ErrSettled) {
		t.Errorf("確定済み月の単発削除: err = %v, want ErrSettled", err)
	}
	// 毎月の予約は特定月に属さないため、確定済みの月があっても編集・削除できる。
	if _, err := f.reserve.Update(ctx, recurring.ID, application.RegisterReservationInput{MemberID: wife, Description: "電気代"}); err != nil {
		t.Errorf("毎月の予約の更新: %v", err)
	}
	if err := f.reserve.Delete(ctx, recurring.ID); err != nil {
		t.Errorf("毎月の予約の削除: %v", err)
	}
}

func TestReservationClosingDayPeriod(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.settings.UpdateClosingDay(ctx, 15); err != nil {
		t.Fatalf("UpdateClosingDay: %v", err)
	}
	r, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "電気代"})
	// 締め日=15 なら 6/20 は 7月分。
	if _, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 5000, Date: "2026-06-20"}); err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
	if s := statusOf(t, f, "2026-07", r.ID); s.Status != domain.ReservationFulfilled {
		t.Errorf("2026-07: status = %v, want fulfilled", s.Status)
	}
	if s := statusOf(t, f, "2026-06", r.ID); s.Status != domain.ReservationPending {
		t.Errorf("2026-06: status = %v, want pending", s.Status)
	}
}

func TestReservationUpdateFrequency(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	r, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "電気代"})
	state, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 7000, Date: "2026-07-25"})
	if err != nil {
		t.Fatalf("Fulfill: %v", err)
	}
	if _, err := f.reserve.SetSkipped(ctx, r.ID, "2026-08", true); err != nil {
		t.Fatalf("SetSkipped: %v", err)
	}

	// 毎月 → 2026-07 のみ: IDは変わらず、7月の入力済みはそのまま有効。8月からは消える。
	oneOff, err := f.reserve.Update(ctx, r.ID, application.RegisterReservationInput{MemberID: husband, Description: "電気代", Month: "2026-07"})
	if err != nil {
		t.Fatalf("Update(→単発): %v", err)
	}
	if oneOff.IsRecurring() || oneOff.ID != r.ID || len(oneOff.SkippedMonths) != 0 {
		t.Fatalf("oneOff = %+v（同じIDの単発になり、今月はなしは消えるべき）", oneOff)
	}
	if s := statusOf(t, f, "2026-07", r.ID); s.Status != domain.ReservationFulfilled || s.FulfilledIDs[0] != state.FulfilledIDs[0] {
		t.Errorf("7月の入力状況: %+v", s)
	}
	if list, _ := f.reserve.ListForMonth(ctx, "2026-08", ""); len(list) != 0 {
		t.Errorf("単発にした予約が8月に残っている: %+v", list)
	}

	// 2026-07 のみ → 毎月: 7月は入力済みのまま、8月は未入力（単発へ変えた時点で今月はなしは消えている）。
	back, err := f.reserve.Update(ctx, r.ID, application.RegisterReservationInput{MemberID: husband, Description: "電気代", Month: ""})
	if err != nil {
		t.Fatalf("Update(→毎月): %v", err)
	}
	if !back.IsRecurring() || back.ID != r.ID {
		t.Fatalf("back = %+v", back)
	}
	if s := statusOf(t, f, "2026-07", r.ID); s.Status != domain.ReservationFulfilled {
		t.Errorf("7月: %v, want fulfilled", s.Status)
	}
	if s := statusOf(t, f, "2026-08", r.ID); s.Status != domain.ReservationPending {
		t.Errorf("8月: %v, want pending", s.Status)
	}

	// 毎月のまま内容を変えても今月はなしは維持される。
	if _, err := f.reserve.SetSkipped(ctx, r.ID, "2026-09", true); err != nil {
		t.Fatalf("SetSkipped: %v", err)
	}
	if _, err := f.reserve.Update(ctx, r.ID, application.RegisterReservationInput{MemberID: wife, Description: "電気代", Month: ""}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if s := statusOf(t, f, "2026-09", r.ID); s.Status != domain.ReservationSkipped {
		t.Errorf("9月: %v, want skipped", s.Status)
	}
}

// laggingExpenses は一覧（Query）に直前の書き込みが反映されない状況（DynamoDB の結果整合性）を再現する
// テスト用の支出リポジトリ。hidden に入れた支出は FindByMonth に現れない（強い整合性の取得には現れる）。
type laggingExpenses struct {
	*memory.ExpenseRepository
	hidden map[domain.ExpenseID]bool
}

func (r *laggingExpenses) FindByMonth(ctx context.Context, month domain.YearMonth) ([]domain.Expense, error) {
	list, err := r.ExpenseRepository.FindByMonth(ctx, month)
	var visible []domain.Expense
	for _, e := range list {
		if !r.hidden[e.ID] {
			visible = append(visible, e)
		}
	}
	return visible, err
}

func TestReservationFulfillLock(t *testing.T) {
	ctx := context.Background()
	couple, _ := domain.NewCouple(domain.Member{ID: husband, Name: "太郎"}, domain.Member{ID: wife, Name: "花子"})
	expenses := &laggingExpenses{ExpenseRepository: memory.NewExpenseRepository(), hidden: map[domain.ExpenseID]bool{}}
	settings := memory.NewSettingsRepository()
	reserve := application.NewReservationUsecase(couple, memory.NewReservationRepository(), expenses, memory.NewIncomeRepository(), settings, memory.NewSettlementSnapshotRepository(), nil)
	settingsUC := application.NewSettingsUsecase(couple, settings)
	all := func() []domain.Expense {
		list, _ := expenses.ExpenseRepository.FindByMonth(ctx, domain.YearMonthOf(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)))
		return list
	}
	r, _ := reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "電気代"})
	in := func(date string) application.FulfillReservationInput {
		return application.FulfillReservationInput{Month: "2026-07", AmountYen: 7000, Date: date}
	}
	first, err := reserve.Fulfill(ctx, r.ID, in("2026-07-25"))
	if err != nil {
		t.Fatalf("Fulfill: %v", err)
	}

	// 先着の入力が一覧にまだ反映されていない状況で、後着の入力が来る（同時入力・再送）。
	// 事前チェックはすり抜けるが、入力ロックで拒否され、後着側が保存した支出は取り消される。
	expenses.hidden[domain.ExpenseID(first.FulfilledIDs[0])] = true
	if _, err := reserve.Fulfill(ctx, r.ID, in("2026-07-26")); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("後着の入力: err = %v, want ErrValidation", err)
	}
	if list := all(); len(list) != 1 || string(list[0].ID) != first.FulfilledIDs[0] {
		t.Errorf("後着の支出が残っている: %+v", list)
	}
	expenses.hidden = map[domain.ExpenseID]bool{}

	// ロック保持者が削除済み（入力の取り消し）なら古いロックとして引き継ぎ、再入力できる。
	if err := expenses.Delete(ctx, domain.ExpenseID(first.FulfilledIDs[0])); err != nil {
		t.Fatal(err)
	}
	if _, err := reserve.Fulfill(ctx, r.ID, in("2026-07-28")); err != nil {
		t.Fatalf("取り消し後の再入力: %v", err)
	}

	// 締め日の変更でロック保持者が別の精算月（8月）の計上になったら、7月へは改めて入力できる。
	if _, err := settingsUC.UpdateClosingDay(ctx, 25); err != nil {
		t.Fatal(err)
	}
	if _, err := reserve.Fulfill(ctx, r.ID, in("2026-07-10")); err != nil {
		t.Errorf("保持者が別の月へ移った後の入力: %v（7/28 は締め日25で8月分）", err)
	}
}

func TestReservationUpdateFrequencySettledMonth(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, m := range []domain.MemberID{husband, wife} {
		if _, err := f.settlement.InputSalary(ctx, "2026-07", m, 100000); err != nil {
			t.Fatalf("InputSalary: %v", err)
		}
	}
	recurring, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "電気代"})
	oneOff, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "車検", Month: "2026-07"})
	if _, err := f.settlement.SetSettled(ctx, "2026-07", true); err != nil {
		t.Fatalf("SetSettled: %v", err)
	}
	// 確定済みの月へ移す／確定済みの月から出す、のどちらも不可。
	if _, err := f.reserve.Update(ctx, recurring.ID, application.RegisterReservationInput{MemberID: husband, Description: "電気代", Month: "2026-07"}); !errors.Is(err, domain.ErrSettled) {
		t.Errorf("確定済み月への変更: err = %v, want ErrSettled", err)
	}
	if _, err := f.reserve.Update(ctx, oneOff.ID, application.RegisterReservationInput{MemberID: husband, Description: "車検", Month: ""}); !errors.Is(err, domain.ErrSettled) {
		t.Errorf("確定済み月からの変更: err = %v, want ErrSettled", err)
	}
}

func TestReservationSkipOneOffRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "車検", Month: "2026-07"})
	if _, err := f.reserve.SetSkipped(ctx, r.ID, "2026-07", true); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("単発のスキップ: err = %v, want ErrValidation", err)
	}
	// 解除（skipped=false）は単発でも受け付ける（過去に記録されたスキップを消せるように）。
	if s, err := f.reserve.SetSkipped(ctx, r.ID, "2026-07", false); err != nil || s.Status != domain.ReservationPending {
		t.Errorf("単発のスキップ解除 = %+v, %v", s, err)
	}
}

func TestExpenseMoveAfterReservationDeleted(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "車検", Month: "2026-07"})
	state, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 60000, Date: "2026-07-20"})
	if err != nil {
		t.Fatal(err)
	}
	id := domain.ExpenseID(state.FulfilledIDs[0])
	// 予約がある間は別の月へ移せない。
	in := application.RegisterExpenseInput{PaidBy: husband, AmountYen: 60000, Description: "車検", Date: "2026-08-02"}
	if _, err := f.expenses.Update(ctx, id, in); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("予約がある間の移動: err = %v, want ErrValidation", err)
	}
	// 予約を削除すると紐づけが外れ、通常の支出として移せる。
	if err := f.reserve.Delete(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	moved, err := f.expenses.Update(ctx, id, in)
	if err != nil {
		t.Fatalf("予約削除後の移動: %v", err)
	}
	if moved.Reservation.IsFromReservation() || moved.Month().String() != "2026-08" {
		t.Errorf("moved = %+v（紐づけが外れた8月の支出になるべき）", moved)
	}
}

func TestReservationStartMonthInList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r, err := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "電気代", StartMonth: "2026-08"})
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := f.reserve.ListForMonth(ctx, "2026-07", ""); len(list) != 0 {
		t.Errorf("開始月より前の月に現れた: %+v", list)
	}
	if s := statusOf(t, f, "2026-08", r.ID); s.Status != domain.ReservationPending {
		t.Errorf("開始月: %v, want pending", s.Status)
	}
	if _, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 1, Date: "2026-07-01"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("開始月より前への入力: err = %v, want ErrValidation", err)
	}
}

func TestReservationLockFollowsExpenseIDChange(t *testing.T) {
	ctx := context.Background()
	couple, _ := domain.NewCouple(domain.Member{ID: husband, Name: "太郎"}, domain.Member{ID: wife, Name: "花子"})
	expenses := &laggingExpenses{ExpenseRepository: memory.NewExpenseRepository(), hidden: map[domain.ExpenseID]bool{}}
	settings := memory.NewSettingsRepository()
	snapshots := memory.NewSettlementSnapshotRepository()
	reservations := memory.NewReservationRepository()
	reserve := application.NewReservationUsecase(couple, reservations, expenses, memory.NewIncomeRepository(), settings, snapshots, nil)
	expenseUC := application.NewExpenseUsecase(couple, expenses, settings, snapshots, reservations, nil)
	if _, err := application.NewSettingsUsecase(couple, settings).UpdateClosingDay(ctx, 25); err != nil {
		t.Fatal(err)
	}
	r, _ := reserve.Register(ctx, application.RegisterReservationInput{Kind: "expense", MemberID: husband, Description: "電気代"})
	// 締め日=25 なら 7/26 は 8月分。
	state, err := reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-08", AmountYen: 7000, Date: "2026-07-26"})
	if err != nil {
		t.Fatal(err)
	}
	// 同じ 8月分のまま暦月だけ変わる日付へ編集すると支出IDが変わる。ロックも新IDへ移るべき。
	moved, err := expenseUC.Update(ctx, domain.ExpenseID(state.FulfilledIDs[0]), application.RegisterExpenseInput{PaidBy: husband, AmountYen: 7000, Description: "電気代", Date: "2026-08-05"})
	if err != nil {
		t.Fatal(err)
	}
	if string(moved.ID) == state.FulfilledIDs[0] {
		t.Fatalf("支出IDが変わっていない: %s", moved.ID)
	}
	// 一覧にまだ反映されていない状況での二重入力は、移ったロックで拒否される。
	expenses.hidden[moved.ID] = true
	if _, err := reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-08", AmountYen: 7000, Date: "2026-08-06"}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("IDが変わった後の二重入力: err = %v, want ErrValidation", err)
	}
}

func TestIncomeUnlinkAfterReservationDeleted(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r, _ := f.reserve.Register(ctx, application.RegisterReservationInput{Kind: "income", MemberID: wife, Description: "賞与", Month: "2026-07"})
	state, err := f.reserve.Fulfill(ctx, r.ID, application.FulfillReservationInput{Month: "2026-07", AmountYen: 300000})
	if err != nil {
		t.Fatal(err)
	}
	in := application.RegisterIncomeInput{MemberID: wife, AmountYen: 310000, Description: "賞与"}
	kept, err := f.income.Update(ctx, domain.IncomeID(state.FulfilledIDs[0]), in)
	if err != nil || !kept.Reservation.Is(r.ID) {
		t.Fatalf("予約がある間は紐づけを維持する: %+v, %v", kept, err)
	}
	if err := f.reserve.Delete(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	unlinked, err := f.income.Update(ctx, kept.ID, in)
	if err != nil || unlinked.Reservation.IsFromReservation() {
		t.Errorf("予約削除後は紐づけを外す: %+v, %v", unlinked, err)
	}
}
