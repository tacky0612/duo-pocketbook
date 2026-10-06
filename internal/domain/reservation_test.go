package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

func mustYM(t *testing.T, s string) domain.YearMonth {
	t.Helper()
	ym, err := domain.ParseYearMonth(s)
	if err != nil {
		t.Fatalf("ParseYearMonth(%q): %v", s, err)
	}
	return ym
}

func TestReservationID(t *testing.T) {
	if id := domain.NewReservationID("abc"); id != "rsv_abc" {
		t.Errorf("NewReservationID = %q, want rsv_abc（頻度に依存しない）", id)
	}
}

func TestNewReservationValidation(t *testing.T) {
	cases := []struct {
		name   string
		kind   domain.ReservationKind
		member domain.MemberID
		desc   string
	}{
		{"不明な種別", "transfer", "taro", "電気代"},
		{"メンバー未指定", domain.ReservationKindExpense, "", "電気代"},
		{"内容が空", domain.ReservationKindExpense, "taro", "  "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := domain.NewReservation("rsv_x", c.kind, c.member, c.desc, domain.YearMonth{}, domain.YearMonth{})
			if !errors.Is(err, domain.ErrValidation) {
				t.Errorf("err = %v, want ErrValidation", err)
			}
		})
	}

	r, err := domain.NewReservation("rsv_x", domain.ReservationKindExpense, "taro", " 電気代 ", domain.YearMonth{}, domain.YearMonth{})
	if err != nil {
		t.Fatalf("NewReservation: %v", err)
	}
	if r.Description != "電気代" || !r.IsRecurring() {
		t.Errorf("reservation = %+v", r)
	}
}

func TestReservationAppliesTo(t *testing.T) {
	oct, nov := mustYM(t, "2026-10"), mustYM(t, "2026-11")
	recurring := domain.Reservation{ID: "rsv_a"}
	oneOff := domain.Reservation{ID: "rsv_b", Month: oct}
	if !recurring.AppliesTo(oct) || !recurring.AppliesTo(nov) {
		t.Error("毎月の予約はすべての月に存在する")
	}
	if !oneOff.AppliesTo(oct) || oneOff.AppliesTo(nov) {
		t.Error("単発の予約は対象月にのみ存在する")
	}
}

func TestResolveReservations(t *testing.T) {
	oct := mustYM(t, "2026-10")
	reservations := []domain.Reservation{
		// 入力済みはスキップより優先される。
		{ID: "rsv_power", Kind: domain.ReservationKindExpense, MemberID: "taro", Description: "電気代", SkippedMonths: []domain.YearMonth{oct}},
		{ID: "rsv_water", Kind: domain.ReservationKindExpense, MemberID: "taro", Description: "水道代", SkippedMonths: []domain.YearMonth{oct}},
		{ID: "rsv_gas", Kind: domain.ReservationKindExpense, MemberID: "hanako", Description: "ガス代"},
		{ID: "rsv_bonus", Kind: domain.ReservationKindIncome, MemberID: "hanako", Description: "賞与", Month: oct},
		{ID: "rsv_car", Kind: domain.ReservationKindExpense, MemberID: "taro", Description: "車検", Month: oct.Next()},
		// 支出の予約に紐づく「収入」は入力済みとみなさない（種別の取り違え防止）。
		{ID: "rsv_misc", Kind: domain.ReservationKindExpense, MemberID: "taro", Description: "雑費"},
	}
	expenses := []domain.Expense{
		{ID: "2026-10_e2", Amount: 3000, Reservation: domain.NewReservationRef("rsv_power")},
		{ID: "2026-10_e1", Amount: 5000, Reservation: domain.NewReservationRef("rsv_power")},
		{ID: "2026-10_e3", Amount: 1000}, // 予約と無関係
	}
	incomes := []domain.Income{
		{ID: "2026-10_i1", Amount: 200000, Reservation: domain.NewReservationRef("rsv_bonus")},
		{ID: "2026-10_i2", Amount: 100, Reservation: domain.NewReservationRef("rsv_misc")},
	}
	states := domain.ResolveReservations(oct, reservations, expenses, incomes)
	got := map[domain.ReservationID]domain.ReservationState{}
	for _, s := range states {
		got[s.Reservation.ID] = s
	}
	if len(states) != 5 {
		t.Fatalf("len(states) = %d, want 5（別月の単発は含めない）", len(states))
	}
	if _, ok := got["rsv_car"]; ok {
		t.Error("別月の単発予約が含まれている")
	}

	power := got["rsv_power"]
	if power.Status != domain.ReservationFulfilled || power.FulfilledAmount != 8000 || len(power.FulfilledIDs) != 2 || power.FulfilledIDs[0] != "2026-10_e1" {
		t.Errorf("電気代 = %+v, want fulfilled 8000 / 2026-10_e1", power)
	}
	if s := got["rsv_water"].Status; s != domain.ReservationSkipped {
		t.Errorf("水道代 = %v, want skipped", s)
	}
	if s := got["rsv_gas"].Status; s != domain.ReservationPending {
		t.Errorf("ガス代 = %v, want pending", s)
	}
	if b := got["rsv_bonus"]; b.Status != domain.ReservationFulfilled || b.FulfilledAmount != 200000 {
		t.Errorf("賞与 = %+v, want fulfilled 200000", b)
	}
	if s := got["rsv_misc"].Status; s != domain.ReservationPending {
		t.Errorf("雑費 = %v, want pending（種別違いの紐づけは無視）", s)
	}

	pending := domain.PendingReservations(states)
	if len(pending) != 2 {
		t.Errorf("len(pending) = %d, want 2", len(pending))
	}
}

func TestReservationRef(t *testing.T) {
	var none domain.ReservationRef
	if _, ok := none.ID(); ok || none.IsFromReservation() || none.String() != "" {
		t.Errorf("ゼロ値は予約に由来しない参照であるべき: %+v", none)
	}
	if domain.NewReservationRef("") != none {
		t.Error("空の予約IDからはゼロ値の参照になるべき")
	}
	ref := domain.NewReservationRef("rsv_a")
	if id, ok := ref.ID(); !ok || id != "rsv_a" || !ref.IsFromReservation() || ref.String() != "rsv_a" {
		t.Errorf("ref = %+v", ref)
	}
}

func TestReservationFulfill(t *testing.T) {
	jul := mustYM(t, "2026-07")
	now := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
	date := func(s string) time.Time {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	expenseRsv := domain.Reservation{ID: "rsv_power", Kind: domain.ReservationKindExpense, MemberID: "taro", Description: "電気代"}
	incomeRsv := domain.Reservation{ID: "rsv_bonus", Kind: domain.ReservationKindIncome, MemberID: "hanako", Description: "賞与", Month: jul}

	e, err := expenseRsv.FulfillAsExpense("x", jul, domain.DefaultClosingDay, 7820, date("2026-07-25"), now)
	if err != nil {
		t.Fatalf("FulfillAsExpense: %v", err)
	}
	if id, ok := e.Reservation.ID(); !ok || id != expenseRsv.ID || e.PaidBy != "taro" || e.Description != "電気代" || e.Amount != 7820 {
		t.Errorf("expense = %+v", e)
	}
	// 締め日=15 なら 6/20 は 7月分として入力できる。
	if _, err := expenseRsv.FulfillAsExpense("x", jul, 15, 1, date("2026-06-20"), now); err != nil {
		t.Errorf("締め日考慮: %v", err)
	}

	inc, err := incomeRsv.FulfillAsIncome("y", jul, 300000)
	if err != nil {
		t.Fatalf("FulfillAsIncome: %v", err)
	}
	if id, ok := inc.Reservation.ID(); !ok || id != incomeRsv.ID || inc.IsRecurring() || inc.Month != jul || inc.MemberID != "hanako" {
		t.Errorf("income = %+v", inc)
	}

	invalid := []struct {
		name string
		err  error
	}{
		{"期間外の支出日", func() error {
			_, err := expenseRsv.FulfillAsExpense("x", jul, domain.DefaultClosingDay, 1, date("2026-08-01"), now)
			return err
		}()},
		{"収入の予約を支出として入力", func() error {
			_, err := incomeRsv.FulfillAsExpense("x", jul, domain.DefaultClosingDay, 1, date("2026-07-01"), now)
			return err
		}()},
		{"支出の予約を収入として入力", func() error { _, err := expenseRsv.FulfillAsIncome("y", jul, 1); return err }()},
		{"対象外の月", func() error { _, err := incomeRsv.FulfillAsIncome("y", jul.Next(), 1); return err }()},
		{"金額0", func() error { _, err := incomeRsv.FulfillAsIncome("y", jul, 0); return err }()},
	}
	for _, c := range invalid {
		if !errors.Is(c.err, domain.ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", c.name, c.err)
		}
	}
}

func TestReservationReviseAndSkip(t *testing.T) {
	jul, aug := mustYM(t, "2026-07"), mustYM(t, "2026-08")
	r, err := domain.NewReservation("rsv_a", domain.ReservationKindExpense, "taro", "電気代", domain.YearMonth{}, domain.YearMonth{})
	if err != nil {
		t.Fatal(err)
	}
	// 毎月の予約は月ごとに「今月はなし」にできる（昇順・重複なし）。
	r, err = r.Skip(aug)
	if err != nil {
		t.Fatalf("Skip: %v", err)
	}
	r, _ = r.Skip(jul)
	r, _ = r.Skip(aug)
	if len(r.SkippedMonths) != 2 || r.SkippedMonths[0] != jul || !r.IsSkippedIn(aug) {
		t.Errorf("SkippedMonths = %v", r.SkippedMonths)
	}
	if u := r.Unskip(jul); u.IsSkippedIn(jul) || !u.IsSkippedIn(aug) || !r.IsSkippedIn(jul) {
		t.Error("Unskip は対象月だけを解除し、元の値を変更しない")
	}

	// 毎月のまま変更するとスキップは維持、単発へ変更すると消える。IDと種別は変わらない。
	same, err := r.Revise("hanako", "電気代（東京）", domain.YearMonth{}, domain.YearMonth{})
	if err != nil || same.ID != r.ID || same.Kind != r.Kind || len(same.SkippedMonths) != 2 || same.MemberID != "hanako" {
		t.Errorf("Revise(毎月) = %+v, %v", same, err)
	}
	oneOff, err := r.Revise("taro", "電気代", jul, domain.YearMonth{})
	if err != nil || oneOff.ID != r.ID || oneOff.IsRecurring() || len(oneOff.SkippedMonths) != 0 {
		t.Errorf("Revise(単発) = %+v, %v", oneOff, err)
	}
	// 単発の予約はスキップできず、対象外の月もスキップできない。
	if _, err := oneOff.Skip(jul); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("単発のスキップ: err = %v", err)
	}
	if got := oneOff.RestoreSkippedMonths([]domain.YearMonth{jul}); len(got.SkippedMonths) != 0 {
		t.Error("単発の予約にはスキップを復元しない")
	}
	ref := domain.NewReservationRef("rsv_abc")
	if !ref.Is("rsv_abc") || ref.Is("rsv_x") || (domain.ReservationRef{}).Is("") {
		t.Error("ReservationRef.Is の判定が不正")
	}
}

func TestExpenseEnsureMovableTo(t *testing.T) {
	d := func(s string) time.Time {
		v, err := time.Parse("2006-01-02", s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	plain := domain.Expense{Date: d("2026-07-25")}
	reserved := domain.Expense{Date: d("2026-07-25"), Reservation: domain.NewReservationRef("rsv_a")}

	if err := plain.EnsureMovableTo(d("2026-08-03"), domain.DefaultClosingDay); err != nil {
		t.Errorf("通常の支出は別の月へ移せる: %v", err)
	}
	if err := reserved.EnsureMovableTo(d("2026-07-01"), domain.DefaultClosingDay); err != nil {
		t.Errorf("同じ精算月内の変更はできる: %v", err)
	}
	if err := reserved.EnsureMovableTo(d("2026-08-03"), domain.DefaultClosingDay); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("予約由来の支出を別の月へ: err = %v, want ErrValidation", err)
	}
	// 締め日=15 なら 7/25 と 8/03 はどちらも 8月分なので移せる。
	if err := reserved.EnsureMovableTo(d("2026-08-03"), 15); err != nil {
		t.Errorf("締め日考慮で同じ精算月: %v", err)
	}
}

func TestReservationStateTransitions(t *testing.T) {
	pending := domain.ReservationState{Status: domain.ReservationPending}
	skipped := domain.ReservationState{Status: domain.ReservationSkipped}
	if s := skipped.WithFulfillment("e1", 500); s.Status != domain.ReservationFulfilled || s.FulfilledIDs[0] != "e1" || s.FulfilledAmount != 500 {
		t.Errorf("スキップ中に入力 = %+v", s)
	}
	if s := pending.WithSkip(true); s.Status != domain.ReservationSkipped {
		t.Errorf("スキップ = %v", s.Status)
	}
	if s := skipped.WithSkip(false); s.Status != domain.ReservationPending {
		t.Errorf("スキップ解除 = %v", s.Status)
	}
	fulfilled := pending.WithFulfillment("e1", 500)
	if s := fulfilled.WithSkip(false); s.Status != domain.ReservationFulfilled {
		t.Errorf("入力済みはスキップ解除で変わらない: %v", s.Status)
	}
}

func TestReservationEnsureSkippable(t *testing.T) {
	if err := (domain.Reservation{ID: "rsv_a"}).EnsureSkippable(); err != nil {
		t.Errorf("毎月の予約はスキップできる: %v", err)
	}
	oneOff := domain.Reservation{ID: "rsv_b", Month: mustYM(t, "2026-07")}
	if err := oneOff.EnsureSkippable(); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("単発の予約: err = %v, want ErrValidation", err)
	}
}

func TestReservationStartMonth(t *testing.T) {
	sep, oct, nov := mustYM(t, "2026-09"), mustYM(t, "2026-10"), mustYM(t, "2026-11")
	if !sep.Before(oct) || oct.Before(oct) || nov.Before(oct) || !mustYM(t, "2025-12").Before(sep) {
		t.Error("YearMonth.Before の判定が不正")
	}
	r, err := domain.NewReservation("rsv_a", domain.ReservationKindExpense, "taro", "電気代", domain.YearMonth{}, oct)
	if err != nil {
		t.Fatal(err)
	}
	// 開始月より前の月には存在しない。
	if r.AppliesTo(sep) || !r.AppliesTo(oct) || !r.AppliesTo(nov) {
		t.Error("毎月の予約は開始月以降にだけ存在する")
	}
	if _, err := r.Skip(sep); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("開始月より前のスキップ: err = %v", err)
	}
	// 単発の予約は開始月を持たない。
	oneOff, _ := domain.NewReservation("rsv_b", domain.ReservationKindExpense, "taro", "車検", nov, oct)
	if !oneOff.StartMonth.IsZero() {
		t.Errorf("単発の StartMonth = %v, want zero", oneOff.StartMonth)
	}
	// 単発 → 毎月（開始月未指定）は元の対象月が開始月になり、毎月のまま（未指定）は開始月を維持する。
	back, _ := oneOff.Revise("taro", "車検", domain.YearMonth{}, domain.YearMonth{})
	if back.StartMonth != nov {
		t.Errorf("単発→毎月の開始月 = %v, want %v", back.StartMonth, nov)
	}
	kept, _ := r.Revise("taro", "電気代", domain.YearMonth{}, domain.YearMonth{})
	if kept.StartMonth != oct {
		t.Errorf("毎月のままの開始月 = %v, want %v", kept.StartMonth, oct)
	}
	moved, _ := r.Revise("taro", "電気代", domain.YearMonth{}, sep)
	if moved.StartMonth != sep {
		t.Errorf("開始月の指定 = %v, want %v", moved.StartMonth, sep)
	}
}
