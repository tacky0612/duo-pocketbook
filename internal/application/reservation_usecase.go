package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ReservationUsecase は収入・支出の予約（金額未確定の予定）に関するユースケース。
//
// 予約自体は精算に影響しない。金額を入力（Fulfill）すると、予約IDを紐づけた共有支出または
// 追加収入（単発）が対象月に登録され、その実データが精算に反映される。
type ReservationUsecase struct {
	couple       domain.Couple
	reservations ReservationRepository
	expenses     ExpenseRepository
	incomes      IncomeRepository
	settings     SettingsRepository
	snapshots    SettlementSnapshotRepository
	now          func() time.Time
}

// NewReservationUsecase は ReservationUsecase を生成する。now が nil の場合は time.Now を使う。
func NewReservationUsecase(couple domain.Couple, reservations ReservationRepository, expenses ExpenseRepository, incomes IncomeRepository, settings SettingsRepository, snapshots SettlementSnapshotRepository, now func() time.Time) *ReservationUsecase {
	if now == nil {
		now = time.Now
	}
	return &ReservationUsecase{couple: couple, reservations: reservations, expenses: expenses, incomes: incomes, settings: settings, snapshots: snapshots, now: now}
}

// RegisterReservationInput は予約の登録・更新の入力。
// Month が空文字なら毎月、"YYYY-MM" ならその精算月のみの単発として扱う。
type RegisterReservationInput struct {
	Kind               string // "expense" | "income"（更新時は無視）
	MemberID           domain.MemberID
	Description        string
	EstimatedAmountYen int64 // 0 は未定
	Month              string
	// StartMonth は毎月の予約の開始月（"YYYY-MM"）。これより前の月には予約が現れない。
	// 登録時に空なら制限なし。更新時に空なら、毎月のままは現在の開始月、単発から毎月へ変えるなら元の対象月を使う。
	StartMonth string
}

// Register は予約を登録する。単発の登録先が確定済みの月なら拒否する。
func (u *ReservationUsecase) Register(ctx context.Context, in RegisterReservationInput) (domain.Reservation, error) {
	kind, err := domain.ParseReservationKind(in.Kind)
	if err != nil {
		return domain.Reservation{}, err
	}
	if !u.couple.Contains(in.MemberID) {
		return domain.Reservation{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.MemberID)
	}
	month, err := domain.ParseOptionalYearMonth(in.Month)
	if err != nil {
		return domain.Reservation{}, err
	}
	start, err := domain.ParseOptionalYearMonth(in.StartMonth)
	if err != nil {
		return domain.Reservation{}, err
	}
	if err := u.ensureMonthsNotSettled(ctx, month); err != nil {
		return domain.Reservation{}, err
	}
	r, err := domain.NewReservation(string(domain.NewReservationID(newIDSuffix())), kind, in.MemberID, in.Description, domain.Money(in.EstimatedAmountYen), month, start)
	if err != nil {
		return domain.Reservation{}, err
	}
	if err := u.reservations.Save(ctx, r); err != nil {
		return domain.Reservation{}, fmt.Errorf("予約の保存に失敗しました: %w", err)
	}
	return r, nil
}

// Update は既存の予約のメンバー・内容・見込み額・頻度・開始月を更新する（IDと種別は維持）。
//
// 頻度は in.Month で指定する（空文字なら毎月、"YYYY-MM" ならその精算月のみ）。IDは頻度に依存しないため
// 予約1件の上書きで完結し、予約に紐づく支出・収入はそのまま有効。単発へ変更すると「今月はなし」の記録は消える。
func (u *ReservationUsecase) Update(ctx context.Context, id domain.ReservationID, in RegisterReservationInput) (domain.Reservation, error) {
	existing, err := u.reservations.FindByID(ctx, id)
	if err != nil {
		return domain.Reservation{}, err
	}
	if !u.couple.Contains(in.MemberID) {
		return domain.Reservation{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.MemberID)
	}
	month, err := domain.ParseOptionalYearMonth(in.Month)
	if err != nil {
		return domain.Reservation{}, err
	}
	start, err := domain.ParseOptionalYearMonth(in.StartMonth)
	if err != nil {
		return domain.Reservation{}, err
	}
	// 変更前・変更後（単発の場合）の月が確定済みなら編集を拒否する
	// （頻度変更で確定済みの月へ移す／から出すのも不可）。毎月（月なし）は対象外。
	if err := u.ensureMonthsNotSettled(ctx, existing.Month, month); err != nil {
		return domain.Reservation{}, err
	}
	r, err := existing.Revise(in.MemberID, in.Description, domain.Money(in.EstimatedAmountYen), month, start)
	if err != nil {
		return domain.Reservation{}, err
	}
	if err := u.reservations.Save(ctx, r); err != nil {
		return domain.Reservation{}, fmt.Errorf("予約の更新に失敗しました: %w", err)
	}
	return r, nil
}

// Delete は予約を削除する。単発でその月が確定済みなら拒否する。
// 予約から登録済みの支出・収入は実データとして残す（「今月はなし」の記録は予約と一緒に消える）。
func (u *ReservationUsecase) Delete(ctx context.Context, id domain.ReservationID) error {
	existing, err := u.reservations.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := u.ensureMonthsNotSettled(ctx, existing.Month); err != nil {
		return err
	}
	if err := u.reservations.Delete(ctx, id); err != nil {
		return fmt.Errorf("予約の削除に失敗しました: %w", err)
	}
	return nil
}

// ListForMonth は指定精算月に存在する予約（毎月分＋当月単発分）と入力状況を返す。
// kind が空でなければその種別だけに絞り込む。毎月を先に、単発を後に、各グループ内は内容の昇順で並べる。
func (u *ReservationUsecase) ListForMonth(ctx context.Context, month, kind string) ([]domain.ReservationState, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	var filter domain.ReservationKind
	if kind != "" {
		if filter, err = domain.ParseReservationKind(kind); err != nil {
			return nil, err
		}
	}
	all, err := u.reservations.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("予約の取得に失敗しました: %w", err)
	}
	var list []domain.Reservation
	for _, r := range all {
		if r.AppliesTo(ym) && (filter == "" || r.Kind == filter) {
			list = append(list, r)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].IsRecurring() != list[j].IsRecurring() {
			return list[i].IsRecurring()
		}
		return list[i].Description < list[j].Description
	})
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return nil, err
	}
	return u.resolve(ctx, ym, closingDay, list)
}

// FulfillReservationInput は予約の金額入力（実データ登録）の入力。
type FulfillReservationInput struct {
	Month     string // 対象の精算月（YYYY-MM）
	AmountYen int64  // 確定した金額
	Date      string // 支出の予約のみ必須（YYYY-MM-DD）。精算月 Month の精算期間内の日付
}

// Fulfill は予約の金額を入力し、対象月の共有支出（支出の予約）または追加収入の単発（収入の予約）
// として登録する。登録した実データには予約IDを紐づけ、入力状況は「入力済み」になる。
// 対象月が「今月はなし」だった場合は解除する。既に入力済みなら拒否する。
//
// 二重入力（同時入力や再送）は「予約×精算月」の入力ロックの条件付き取得で防ぐ。DynamoDB の Query は
// 結果整合性のため、事前の入力済みチェックだけでは直前の書き込みを見落とすことがあるため。
// 返す入力状況は、書き込み後に読み直さず書き込み前の状況から組み立てる（同じ理由）。
func (u *ReservationUsecase) Fulfill(ctx context.Context, id domain.ReservationID, in FulfillReservationInput) (domain.ReservationState, error) {
	r, ym, err := u.findForMonth(ctx, id, in.Month)
	if err != nil {
		return domain.ReservationState{}, err
	}
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return domain.ReservationState{}, err
	}
	state, err := u.stateOf(ctx, ym, closingDay, r)
	if err != nil {
		return domain.ReservationState{}, err
	}
	if state.Status == domain.ReservationFulfilled {
		return domain.ReservationState{}, alreadyFulfilled(ym)
	}

	// 実データを組み立てて保存し、その後で入力ロックを取る。ロックが取れなければ保存した実データを取り消す。
	// （先に保存しておくことで、競合した側がロック保持者の実データの有無を確認できる。）
	var actual fulfilledActual
	switch r.Kind {
	case domain.ReservationKindExpense:
		date, err := time.Parse("2006-01-02", in.Date)
		if err != nil {
			return domain.ReservationState{}, fmt.Errorf("%w: 支出日は YYYY-MM-DD 形式で指定してください: %q", domain.ErrValidation, in.Date)
		}
		e, err := r.FulfillAsExpense(newIDSuffix(), ym, closingDay, domain.Money(in.AmountYen), date, u.now())
		if err != nil {
			return domain.ReservationState{}, err
		}
		if err := u.expenses.Save(ctx, e); err != nil {
			return domain.ReservationState{}, fmt.Errorf("支出の保存に失敗しました: %w", err)
		}
		actual = fulfilledActual{id: string(e.ID), amount: e.Amount, remove: func() error { return u.expenses.Delete(ctx, e.ID) }}
	case domain.ReservationKindIncome:
		inc, err := r.FulfillAsIncome(newIDSuffix(), ym, domain.Money(in.AmountYen))
		if err != nil {
			return domain.ReservationState{}, err
		}
		if err := u.incomes.Save(ctx, inc); err != nil {
			return domain.ReservationState{}, fmt.Errorf("収入の保存に失敗しました: %w", err)
		}
		actual = fulfilledActual{id: string(inc.ID), amount: inc.Amount, remove: func() error { return u.incomes.Delete(ctx, inc.ID) }}
	}
	if err := u.lockFulfillment(ctx, r, ym, closingDay, actual.id); err != nil {
		if rmErr := actual.remove(); rmErr != nil {
			return domain.ReservationState{}, errors.Join(err, fmt.Errorf("登録した実データの取り消しに失敗しました: %w", rmErr))
		}
		return domain.ReservationState{}, err
	}
	if r.IsSkippedIn(ym) {
		u.clearSkip(ctx, r.ID, ym)
	}
	return state.WithFulfillment(actual.id, actual.amount), nil
}

// clearSkip は金額入力後に対象月の「今月はなし」を解除する。
//
// 入力済みは今月はなしより優先されるため、解除できなくても入力状況は正しく「入力済み」になる。そのため失敗しても
// 入力自体は成功として扱い、ログだけ残す（入力済みの支出・収入とロックは保存済みで、エラーを返すと再試行が
// 「入力済み」で拒否されてしまう）。解除は月単位の削除で行い、同時に行われた予約の編集を上書きしない。
func (u *ReservationUsecase) clearSkip(ctx context.Context, id domain.ReservationID, ym domain.YearMonth) {
	if err := u.reservations.RemoveSkip(ctx, id, ym); err != nil {
		slog.Warn("金額入力後の「今月はなし」の解除に失敗しました", "reservationId", id, "month", ym.String(), "error", err)
	}
}

// fulfilledActual は金額入力で登録した実データ（支出または収入）。
type fulfilledActual struct {
	id     string
	amount domain.Money
	remove func() error // 入力ロックを取れなかったときの取り消し
}

// lockFulfillment は予約×精算月の入力ロックを target で取得する。
//
// 既に他の実データがロックを持っていても、それが「存在し、この予約に紐づき、この精算月に計上されている」
// ときだけ入力済みとして拒否する。それ以外（入力の取り消しで削除された、締め日の変更で別の月の計上に
// なった、日付変更でIDが変わった等）は古いロックとみなして引き継ぐ。
func (u *ReservationUsecase) lockFulfillment(ctx context.Context, r domain.Reservation, ym domain.YearMonth, cd domain.ClosingDay, target string) error {
	acquired, current, err := u.reservations.AcquireFulfillment(ctx, r.ID, ym, target)
	if err != nil {
		return fmt.Errorf("入力ロックの取得に失敗しました: %w", err)
	}
	if acquired {
		return nil
	}
	held, err := u.holdsFulfillment(ctx, r, ym, cd, current)
	if err != nil {
		return err
	}
	if held {
		return alreadyFulfilled(ym)
	}
	replaced, err := u.reservations.ReplaceFulfillment(ctx, r.ID, ym, current, target)
	if err != nil {
		return fmt.Errorf("入力ロックの取得に失敗しました: %w", err)
	}
	if !replaced {
		return alreadyFulfilled(ym) // 同時に別の入力がロックを引き継いだ
	}
	return nil
}

// holdsFulfillment は実データ id が予約 r の精算月 ym の入力として有効か（存在し、r に紐づき、ym に計上されるか）を返す。
// 直前に保存された実データを見落とさないよう、強い整合性で読む。
func (u *ReservationUsecase) holdsFulfillment(ctx context.Context, r domain.Reservation, ym domain.YearMonth, cd domain.ClosingDay, id string) (bool, error) {
	var (
		held bool
		err  error
	)
	switch r.Kind {
	case domain.ReservationKindExpense:
		if _, monthErr := domain.ExpenseID(id).Month(); monthErr != nil {
			return false, nil // 不正なIDは無効なロック扱い
		}
		var e domain.Expense
		if e, err = u.expenses.FindByIDConsistent(ctx, domain.ExpenseID(id)); err == nil {
			held = e.Reservation.Is(r.ID) && cd.SettlementMonth(e.Date) == ym
		}
	case domain.ReservationKindIncome:
		var inc domain.Income
		if inc, err = u.incomes.FindByIDConsistent(ctx, domain.IncomeID(id)); err == nil {
			held = inc.Reservation.Is(r.ID) && inc.Month == ym
		}
	}
	switch {
	case err == nil:
		return held, nil
	case errors.Is(err, ErrNotFound):
		return false, nil
	default:
		return false, fmt.Errorf("実データの確認に失敗しました: %w", err)
	}
}

func alreadyFulfilled(ym domain.YearMonth) error {
	return fmt.Errorf("%w: この予約は%sに入力済みです", domain.ErrValidation, ym)
}

// SetSkipped は毎月の予約を対象月で「今月はなし」にする（skipped=false で解除）。
// スキップ済みの予約は未入力として扱わない。入力済みの予約と、単発の予約はスキップできない。
// 返す入力状況は Fulfill と同じく書き込み前の状況から組み立てる。
func (u *ReservationUsecase) SetSkipped(ctx context.Context, id domain.ReservationID, month string, skipped bool) (domain.ReservationState, error) {
	r, ym, err := u.findForMonth(ctx, id, month)
	if err != nil {
		return domain.ReservationState{}, err
	}
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return domain.ReservationState{}, err
	}
	state, err := u.stateOf(ctx, ym, closingDay, r)
	if err != nil {
		return domain.ReservationState{}, err
	}
	// 記録の更新は月単位の追加・削除で行い、予約全体は上書きしない（同時に行われた他の月のスキップや編集を失わないため）。
	next := r.Unskip(ym)
	update := u.reservations.RemoveSkip
	if skipped {
		if next, err = r.Skip(ym); err != nil {
			return domain.ReservationState{}, err
		}
		if state.Status == domain.ReservationFulfilled {
			return domain.ReservationState{}, fmt.Errorf("%w: 入力済みの予約はスキップできません", domain.ErrValidation)
		}
		update = u.reservations.AddSkip
	}
	if err := update(ctx, r.ID, ym); err != nil {
		return domain.ReservationState{}, fmt.Errorf("「今月はなし」の更新に失敗しました: %w", err)
	}
	state.Reservation = next
	return state.WithSkip(skipped), nil
}

// findForMonth は予約と対象精算月を取得し、予約がその月に存在すること・月が未確定であることを確認する。
func (u *ReservationUsecase) findForMonth(ctx context.Context, id domain.ReservationID, month string) (domain.Reservation, domain.YearMonth, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return domain.Reservation{}, domain.YearMonth{}, err
	}
	r, err := u.reservations.FindByID(ctx, id)
	if err != nil {
		return domain.Reservation{}, domain.YearMonth{}, err
	}
	if !r.AppliesTo(ym) {
		return domain.Reservation{}, domain.YearMonth{}, fmt.Errorf("%w: この予約は %s には存在しません", domain.ErrValidation, ym)
	}
	if err := ensureMonthNotSettled(ctx, u.snapshots, ym); err != nil {
		return domain.Reservation{}, domain.YearMonth{}, err
	}
	return r, ym, nil
}

// ensureMonthsNotSettled は指定した月（ゼロ値は無視）のいずれかが確定済みなら domain.ErrSettled を返す。
func (u *ReservationUsecase) ensureMonthsNotSettled(ctx context.Context, months ...domain.YearMonth) error {
	for _, m := range months {
		if m.IsZero() {
			continue
		}
		if err := ensureMonthNotSettled(ctx, u.snapshots, m); err != nil {
			return err
		}
	}
	return nil
}

// stateOf は予約1件の対象月における入力状況を返す。
func (u *ReservationUsecase) stateOf(ctx context.Context, ym domain.YearMonth, cd domain.ClosingDay, r domain.Reservation) (domain.ReservationState, error) {
	states, err := u.resolve(ctx, ym, cd, []domain.Reservation{r})
	if err != nil {
		return domain.ReservationState{}, err
	}
	return states[0], nil
}

// resolve は照合に必要な実データを集めて予約の入力状況を判定する。
// 読み取り回数を抑えるため、支出は支出の予約、収入は収入の予約が list にあるときだけ取得する。
func (u *ReservationUsecase) resolve(ctx context.Context, ym domain.YearMonth, cd domain.ClosingDay, list []domain.Reservation) ([]domain.ReservationState, error) {
	if len(list) == 0 {
		return nil, nil
	}
	var needExpenses, needIncomes bool
	for _, r := range list {
		switch r.Kind {
		case domain.ReservationKindExpense:
			needExpenses = true
		case domain.ReservationKindIncome:
			needIncomes = true
		}
	}
	var (
		expenses []domain.Expense
		incomes  []domain.Income
		err      error
	)
	if needExpenses {
		if expenses, err = expensesForSettlementMonth(ctx, u.expenses, ym, cd); err != nil {
			return nil, fmt.Errorf("支出の取得に失敗しました: %w", err)
		}
	}
	if needIncomes {
		// 予約から登録される収入は常に単発なので、当月単発分だけを照合すればよい。
		if incomes, err = u.incomes.FindByMonth(ctx, ym); err != nil {
			return nil, fmt.Errorf("収入の取得に失敗しました: %w", err)
		}
	}
	return domain.ResolveReservations(ym, list, expenses, incomes), nil
}
