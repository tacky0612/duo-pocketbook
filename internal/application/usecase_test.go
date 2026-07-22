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

const (
	husband = domain.MemberID("taro")
	wife    = domain.MemberID("hanako")
)

type fixture struct {
	expenses   *application.ExpenseUsecase
	settlement *application.SettlementUsecase
	settings   *application.SettingsUsecase
	recurring  *application.RecurringExpenseUsecase
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	couple, err := domain.NewCouple(
		domain.Member{ID: husband, Name: "太郎"},
		domain.Member{ID: wife, Name: "花子"},
	)
	if err != nil {
		t.Fatalf("NewCouple: %v", err)
	}
	expenseRepo := memory.NewExpenseRepository()
	incomeRepo := memory.NewIncomeRepository()
	recurringRepo := memory.NewRecurringExpenseRepository()
	settingsRepo := memory.NewSettingsRepository()
	statusRepo := memory.NewSettlementStatusRepository()
	now := func() time.Time { return time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC) }
	return fixture{
		expenses:   application.NewExpenseUsecase(couple, expenseRepo, now),
		settlement: application.NewSettlementUsecase(couple, expenseRepo, incomeRepo, recurringRepo, settingsRepo, statusRepo),
		settings:   application.NewSettingsUsecase(couple, settingsRepo),
		recurring:  application.NewRecurringExpenseUsecase(couple, recurringRepo),
	}
}

func TestExpenseRegisterAndList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	e, err := f.expenses.Register(ctx, application.RegisterExpenseInput{
		PaidBy: husband, AmountYen: 3000, Description: "食費", Date: "2026-07-10",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if e.Month().String() != "2026-07" {
		t.Errorf("Month = %v", e.Month())
	}

	if _, err := f.expenses.Register(ctx, application.RegisterExpenseInput{
		PaidBy: wife, AmountYen: 5000, Description: "日用品", Date: "2026-07-12",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	list, err := f.expenses.ListByMonth(ctx, "2026-07")
	if err != nil {
		t.Fatalf("ListByMonth: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
	// 日付降順
	if list[0].Description != "日用品" || list[1].Description != "食費" {
		t.Errorf("並び順が不正: %v, %v", list[0].Description, list[1].Description)
	}

	// 別の月には現れない
	other, err := f.expenses.ListByMonth(ctx, "2026-06")
	if err != nil {
		t.Fatalf("ListByMonth: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("len(other) = %d, want 0", len(other))
	}
}

func TestExpenseRegisterValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	cases := map[string]application.RegisterExpenseInput{
		"不明なメンバー": {PaidBy: "unknown", AmountYen: 100, Date: "2026-07-10"},
		"金額0":     {PaidBy: husband, AmountYen: 0, Date: "2026-07-10"},
		"日付形式不正":  {PaidBy: husband, AmountYen: 100, Date: "2026/07/10"},
	}
	for name, in := range cases {
		if _, err := f.expenses.Register(ctx, in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
}

func TestExpenseUpdate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	e, err := f.expenses.Register(ctx, application.RegisterExpenseInput{
		PaidBy: husband, AmountYen: 3000, Description: "食費", Date: "2026-07-10",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// 同月内の更新: IDは維持され内容が変わる
	updated, err := f.expenses.Update(ctx, e.ID, application.RegisterExpenseInput{
		PaidBy: wife, AmountYen: 4500, Description: "食費(訂正)", Date: "2026-07-15",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.ID != e.ID {
		t.Errorf("同月更新でIDが変化した: %s -> %s", e.ID, updated.ID)
	}
	if updated.PaidBy != wife || updated.Amount != 4500 || updated.Description != "食費(訂正)" {
		t.Errorf("更新内容が反映されていない: %+v", updated)
	}
	list, _ := f.expenses.ListByMonth(ctx, "2026-07")
	if len(list) != 1 || list[0].Amount != 4500 {
		t.Errorf("一覧に反映されていない: %+v", list)
	}

	// 別月への更新: 旧月からは消え、新月に移動する
	moved, err := f.expenses.Update(ctx, updated.ID, application.RegisterExpenseInput{
		PaidBy: wife, AmountYen: 4500, Description: "食費(訂正)", Date: "2026-08-03",
	})
	if err != nil {
		t.Fatalf("Update(別月): %v", err)
	}
	if moved.Month().String() != "2026-08" {
		t.Errorf("移動後の月 = %s, want 2026-08", moved.Month())
	}
	july, _ := f.expenses.ListByMonth(ctx, "2026-07")
	aug, _ := f.expenses.ListByMonth(ctx, "2026-08")
	if len(july) != 0 || len(aug) != 1 {
		t.Errorf("移動が反映されていない: 7月=%d件, 8月=%d件", len(july), len(aug))
	}

	// 存在しないIDの更新は ErrNotFound
	if _, err := f.expenses.Update(ctx, "2026-07_missing", application.RegisterExpenseInput{
		PaidBy: husband, AmountYen: 100, Description: "x", Date: "2026-07-01",
	}); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRecurringExpenseUpdate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	e, err := f.recurring.Register(ctx, application.RegisterRecurringExpenseInput{
		PaidBy: husband, AmountYen: 60000, Description: "家賃",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	updated, err := f.recurring.Update(ctx, e.ID, application.RegisterRecurringExpenseInput{
		PaidBy: wife, AmountYen: 62000, Description: "家賃(更新)",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.ID != e.ID || updated.PaidBy != wife || updated.Amount != 62000 || updated.Description != "家賃(更新)" {
		t.Errorf("更新内容が不正: %+v", updated)
	}
	list, _ := f.recurring.List(ctx)
	if len(list) != 1 || list[0].Amount != 62000 {
		t.Errorf("一覧に反映されていない: %+v", list)
	}

	if _, err := f.recurring.Update(ctx, "missing", application.RegisterRecurringExpenseInput{
		PaidBy: husband, AmountYen: 100, Description: "x",
	}); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestExpenseDelete(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	e, err := f.expenses.Register(ctx, application.RegisterExpenseInput{
		PaidBy: husband, AmountYen: 3000, Description: "食費", Date: "2026-07-10",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := f.expenses.Delete(ctx, e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, _ := f.expenses.ListByMonth(ctx, "2026-07")
	if len(list) != 0 {
		t.Errorf("len(list) = %d, want 0", len(list))
	}

	// 存在しないIDの削除は ErrNotFound
	if err := f.expenses.Delete(ctx, e.ID); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSettlementFlow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// ユーザー提示の例を登録する
	if _, err := f.expenses.Register(ctx, application.RegisterExpenseInput{
		PaidBy: husband, AmountYen: 20_000, Description: "家賃(一部)", Date: "2026-07-01",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, err := f.expenses.Register(ctx, application.RegisterExpenseInput{
		PaidBy: wife, AmountYen: 20_000, Description: "食費", Date: "2026-07-05",
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// 収入が揃うまでは ErrIncomeNotReady
	if _, err := f.settlement.GetSettlement(ctx, "2026-07"); !errors.Is(err, domain.ErrIncomeNotReady) {
		t.Fatalf("err = %v, want ErrIncomeNotReady", err)
	}

	if _, err := f.settlement.InputIncome(ctx, "2026-07", husband, 100_000); err != nil {
		t.Fatalf("InputIncome: %v", err)
	}
	if _, err := f.settlement.InputIncome(ctx, "2026-07", wife, 50_000); err != nil {
		t.Fatalf("InputIncome: %v", err)
	}

	got, err := f.settlement.GetSettlement(ctx, "2026-07")
	if err != nil {
		t.Fatalf("GetSettlement: %v", err)
	}
	if got.Transfer == nil {
		t.Fatal("Transfer = nil")
	}
	if got.Transfer.From != husband || got.Transfer.To != wife || got.Transfer.Amount != 25_000 {
		t.Errorf("Transfer = %s→%s %s, want taro→hanako 25000円",
			got.Transfer.From, got.Transfer.To, got.Transfer.Amount)
	}

	// 収入は上書き可能
	if _, err := f.settlement.InputIncome(ctx, "2026-07", wife, 100_000); err != nil {
		t.Fatalf("InputIncome(上書き): %v", err)
	}
	incomes, err := f.settlement.GetIncomes(ctx, "2026-07")
	if err != nil {
		t.Fatalf("GetIncomes: %v", err)
	}
	if len(incomes) != 2 {
		t.Fatalf("len(incomes) = %d, want 2", len(incomes))
	}
}

func TestSettlementWithWeight(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// デフォルトは1:1
	w, err := f.settings.GetWeight(ctx)
	if err != nil {
		t.Fatalf("GetWeight: %v", err)
	}
	if v, _ := w.Of(husband); v != 1 {
		t.Errorf("デフォルト比重 = %d, want 1", v)
	}

	// 2:1 に更新して精算に反映されることを確認
	if _, err := f.settings.UpdateWeight(ctx, application.UpdateWeightInput{
		Weights: map[domain.MemberID]int64{husband: 2, wife: 1},
	}); err != nil {
		t.Fatalf("UpdateWeight: %v", err)
	}

	if _, err := f.settlement.InputIncome(ctx, "2026-07", husband, 100_000); err != nil {
		t.Fatalf("InputIncome: %v", err)
	}
	if _, err := f.settlement.InputIncome(ctx, "2026-07", wife, 50_000); err != nil {
		t.Fatalf("InputIncome: %v", err)
	}
	got, err := f.settlement.GetSettlement(ctx, "2026-07")
	if err != nil {
		t.Fatalf("GetSettlement: %v", err)
	}
	// net夫=10万, net妻=5万 → t=(1*100000-2*50000)/3=0 → 精算不要
	if got.Transfer != nil {
		t.Errorf("Transfer = %+v, want nil", got.Transfer)
	}

	// 不正な比重更新
	if _, err := f.settings.UpdateWeight(ctx, application.UpdateWeightInput{
		Weights: map[domain.MemberID]int64{husband: 0, wife: 1},
	}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
	if _, err := f.settings.UpdateWeight(ctx, application.UpdateWeightInput{
		Weights: map[domain.MemberID]int64{"unknown": 1, wife: 1},
	}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestRecurringExpenseAffectsSettlement(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// 固定費（家賃6万を太郎が毎月立替）を登録
	rc, err := f.recurring.Register(ctx, application.RegisterRecurringExpenseInput{
		PaidBy: husband, AmountYen: 60_000, Description: "家賃",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	list, err := f.recurring.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v", list, err)
	}

	// 収入を両者入力（同額）。固定費が精算に反映されるか確認する。
	if _, err := f.settlement.InputIncome(ctx, "2026-07", husband, 100_000); err != nil {
		t.Fatalf("InputIncome: %v", err)
	}
	if _, err := f.settlement.InputIncome(ctx, "2026-07", wife, 100_000); err != nil {
		t.Fatalf("InputIncome: %v", err)
	}

	// net夫 = 100000 - 60000 = 40000, net妻 = 100000 → t = (40000-100000)/2 = -30000
	// → 妻が夫に30000円振り込む（固定費の立替を折半する形）
	got, err := f.settlement.GetSettlement(ctx, "2026-07")
	if err != nil {
		t.Fatalf("GetSettlement: %v", err)
	}
	if got.TotalExpense != 60_000 {
		t.Errorf("TotalExpense = %s, want 60000円", got.TotalExpense)
	}
	if got.Transfer == nil || got.Transfer.From != wife || got.Transfer.To != husband || got.Transfer.Amount != 30_000 {
		t.Errorf("Transfer = %+v, want hanako→taro 30000", got.Transfer)
	}

	// 削除すると精算対象から外れる
	if err := f.recurring.Delete(ctx, rc.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err = f.settlement.GetSettlement(ctx, "2026-07")
	if err != nil {
		t.Fatalf("GetSettlement: %v", err)
	}
	if got.Transfer != nil {
		t.Errorf("削除後 Transfer = %+v, want nil", got.Transfer)
	}
}

func TestRecurringExpenseValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.recurring.Register(ctx, application.RegisterRecurringExpenseInput{
		PaidBy: "unknown", AmountYen: 1000, Description: "x",
	}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("不明メンバー: err = %v, want ErrValidation", err)
	}
	if _, err := f.recurring.Register(ctx, application.RegisterRecurringExpenseInput{
		PaidBy: husband, AmountYen: 0, Description: "x",
	}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("金額0: err = %v, want ErrValidation", err)
	}
	if _, err := f.recurring.Register(ctx, application.RegisterRecurringExpenseInput{
		PaidBy: husband, AmountYen: 1000, Description: "  ",
	}); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("内容空: err = %v, want ErrValidation", err)
	}
	if err := f.recurring.Delete(ctx, "missing"); !errors.Is(err, application.ErrNotFound) {
		t.Errorf("存在しない削除: err = %v, want ErrNotFound", err)
	}
}

func TestSettlementHistory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// 7月: 収入あり（精算対象）、8月: 収入未入力（スキップ対象）、9月: 収入あり＋精算済み
	if _, err := f.settlement.InputIncome(ctx, "2026-07", husband, 100_000); err != nil {
		t.Fatal(err)
	}
	if _, err := f.settlement.InputIncome(ctx, "2026-07", wife, 60_000); err != nil {
		t.Fatal(err)
	}
	if _, err := f.settlement.InputIncome(ctx, "2026-09", husband, 100_000); err != nil {
		t.Fatal(err)
	}
	if _, err := f.settlement.InputIncome(ctx, "2026-09", wife, 100_000); err != nil {
		t.Fatal(err)
	}
	if _, err := f.settlement.SetSettled(ctx, "2026-09", true); err != nil {
		t.Fatal(err)
	}

	entries, err := f.settlement.History(ctx, "2026-01", "2026-12")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	// 収入のある7月・9月のみ、新しい順で返る
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Settlement.Month.String() != "2026-09" || !entries[0].Settled {
		t.Errorf("entries[0] = %+v, want 2026-09 settled", entries[0].Settlement.Month)
	}
	if entries[1].Settlement.Month.String() != "2026-07" || entries[1].Settled {
		t.Errorf("entries[1] = %+v, want 2026-07 unsettled", entries[1].Settlement.Month)
	}
	// 7月: 収入差40000 → 太郎→花子 20000
	tr := entries[1].Settlement.Transfer
	if tr == nil || tr.From != husband || tr.To != wife || tr.Amount != 20_000 {
		t.Errorf("7月 transfer = %+v, want taro→hanako 20000", tr)
	}

	// from > to はエラー
	if _, err := f.settlement.History(ctx, "2026-12", "2026-01"); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("from>to: err = %v, want ErrValidation", err)
	}
}

func TestSettlementStatus(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// 初期状態は未精算
	settled, err := f.settlement.IsSettled(ctx, "2026-07")
	if err != nil {
		t.Fatalf("IsSettled: %v", err)
	}
	if settled {
		t.Error("初期状態が精算済みになっている")
	}

	// 精算済みに設定
	if _, err := f.settlement.SetSettled(ctx, "2026-07", true); err != nil {
		t.Fatalf("SetSettled: %v", err)
	}
	settled, _ = f.settlement.IsSettled(ctx, "2026-07")
	if !settled {
		t.Error("精算済みが反映されていない")
	}

	// 他の月には影響しない
	other, _ := f.settlement.IsSettled(ctx, "2026-08")
	if other {
		t.Error("別月に精算済みが波及している")
	}

	// 取り消し
	if _, err := f.settlement.SetSettled(ctx, "2026-07", false); err != nil {
		t.Fatalf("SetSettled(false): %v", err)
	}
	settled, _ = f.settlement.IsSettled(ctx, "2026-07")
	if settled {
		t.Error("精算済みの取り消しが反映されていない")
	}

	// 不正な月
	if _, err := f.settlement.SetSettled(ctx, "bad", true); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestMemberProfileNameAndColor(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// デフォルト: 名前は設定値、カラーは既定パレット
	members, err := f.settings.GetMembers(ctx)
	if err != nil {
		t.Fatalf("GetMembers: %v", err)
	}
	if members[0].Name != "太郎" || members[0].Color != "#2563eb" {
		t.Errorf("デフォルト member0 = %+v", members[0])
	}
	if members[1].Color != "#4f46e5" {
		t.Errorf("デフォルト member1 color = %s", members[1].Color)
	}

	// 表示名とカラーを個別に更新しても互いに消えない
	if err := f.settings.UpdateMemberName(ctx, husband, "たろう"); err != nil {
		t.Fatalf("UpdateMemberName: %v", err)
	}
	if err := f.settings.UpdateMemberColor(ctx, husband, "#E11D48"); err != nil {
		t.Fatalf("UpdateMemberColor: %v", err)
	}
	m, err := f.settings.GetMember(ctx, husband)
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if m.Name != "たろう" || m.Color != "#e11d48" {
		t.Errorf("更新後 = %+v, want name=たろう color=#e11d48", m)
	}

	// 不正なカラー
	for _, bad := range []string{"red", "#fff", "#12345g", "2563eb"} {
		if err := f.settings.UpdateMemberColor(ctx, husband, bad); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("UpdateMemberColor(%q) err = %v, want ErrValidation", bad, err)
		}
	}
	// 不明メンバー
	if err := f.settings.UpdateMemberColor(ctx, "unknown", "#123456"); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("不明メンバー: err = %v, want ErrValidation", err)
	}
}

func TestInputIncomeValidation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.settlement.InputIncome(ctx, "2026-07", "unknown", 100); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("不明メンバー: err = %v, want ErrValidation", err)
	}
	if _, err := f.settlement.InputIncome(ctx, "bad-month", husband, 100); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("年月不正: err = %v, want ErrValidation", err)
	}
	if _, err := f.settlement.InputIncome(ctx, "2026-07", husband, -1); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("負の金額: err = %v, want ErrValidation", err)
	}
}
