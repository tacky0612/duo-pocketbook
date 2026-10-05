package application

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ExpenseUsecase は共有支出に関するユースケース。
type ExpenseUsecase struct {
	couple    domain.Couple
	expenses  ExpenseRepository
	settings  SettingsRepository
	snapshots SettlementSnapshotRepository
	// reservations は予約から登録した支出の編集時に、紐づく予約がまだ存在するかの確認に使う。
	reservations ReservationRepository
	now          func() time.Time
}

// NewExpenseUsecase は ExpenseUsecase を生成する。
func NewExpenseUsecase(couple domain.Couple, expenses ExpenseRepository, settings SettingsRepository, snapshots SettlementSnapshotRepository, reservations ReservationRepository, now func() time.Time) *ExpenseUsecase {
	if now == nil {
		now = time.Now
	}
	return &ExpenseUsecase{couple: couple, expenses: expenses, settings: settings, snapshots: snapshots, reservations: reservations, now: now}
}

// RegisterExpenseInput は支出登録の入力。
type RegisterExpenseInput struct {
	PaidBy      domain.MemberID
	AmountYen   int64
	Description string
	Date        string // YYYY-MM-DD
}

// Register は共有支出を登録する。
func (u *ExpenseUsecase) Register(ctx context.Context, in RegisterExpenseInput) (domain.Expense, error) {
	if !u.couple.Contains(in.PaidBy) {
		return domain.Expense{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.PaidBy)
	}
	date, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return domain.Expense{}, fmt.Errorf("%w: 支出日は YYYY-MM-DD 形式で指定してください: %q", domain.ErrValidation, in.Date)
	}
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return domain.Expense{}, err
	}
	if err := ensureMonthNotSettled(ctx, u.snapshots, closingDay.SettlementMonth(date)); err != nil {
		return domain.Expense{}, err
	}
	e, err := domain.NewExpense(newIDSuffix(), in.PaidBy, domain.Money(in.AmountYen), in.Description, date, u.now())
	if err != nil {
		return domain.Expense{}, err
	}
	if err := u.expenses.Save(ctx, e); err != nil {
		return domain.Expense{}, fmt.Errorf("支出の保存に失敗しました: %w", err)
	}
	return e, nil
}

// Update は既存の共有支出の内容を更新する。
// 日付の変更で対象月が変わった場合は、新しい月のIDへ移し替える（旧レコードは削除する）。
func (u *ExpenseUsecase) Update(ctx context.Context, id domain.ExpenseID, in RegisterExpenseInput) (domain.Expense, error) {
	if !u.couple.Contains(in.PaidBy) {
		return domain.Expense{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.PaidBy)
	}
	existing, err := u.expenses.FindByID(ctx, id)
	if err != nil {
		return domain.Expense{}, err
	}
	date, err := time.Parse("2006-01-02", in.Date)
	if err != nil {
		return domain.Expense{}, fmt.Errorf("%w: 支出日は YYYY-MM-DD 形式で指定してください: %q", domain.ErrValidation, in.Date)
	}
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return domain.Expense{}, err
	}
	// 変更前の精算月・変更後の精算月のいずれかが確定済みなら編集を拒否する
	// （確定済みの月から動かす／確定済みの月へ移す、のどちらも不可）。
	if err := ensureMonthNotSettled(ctx, u.snapshots, closingDay.SettlementMonth(existing.Date)); err != nil {
		return domain.Expense{}, err
	}
	if err := ensureMonthNotSettled(ctx, u.snapshots, closingDay.SettlementMonth(date)); err != nil {
		return domain.Expense{}, err
	}
	// 紐づく予約が削除済みなら紐づけを外し、通常の支出として扱う（別の精算月へ移せるようになる）。
	if existing.Reservation, err = liveReservationRef(ctx, u.reservations, existing.Reservation); err != nil {
		return domain.Expense{}, err
	}
	if err := existing.EnsureMovableTo(date, closingDay); err != nil {
		return domain.Expense{}, err
	}
	// 既存IDのサフィックスを引き継ぐ。対象月が同じなら同一IDのまま上書きになる。
	_, suffix, _ := strings.Cut(string(id), "_")
	updated, err := domain.NewExpense(suffix, in.PaidBy, domain.Money(in.AmountYen), in.Description, date, existing.CreatedAt)
	if err != nil {
		return domain.Expense{}, err
	}
	// 予約から登録された支出は、編集後も予約との紐づけを維持する。
	updated.Reservation = existing.Reservation
	if err := u.expenses.Save(ctx, updated); err != nil {
		return domain.Expense{}, fmt.Errorf("支出の更新に失敗しました: %w", err)
	}
	// 月が変わってIDが変化した場合は旧レコードを削除する。
	if updated.ID != id {
		// 予約から登録した支出は、精算月が同じまま暦月だけ変わる（締め日 >= 2）とIDが変わる。予約の入力ロックが
		// 旧IDを指したままだと古いロックとみなされ二重入力を防げないため、旧レコードを削除する前に新IDへ移す
		// （旧IDがまだ存在する間に移すことで、同時入力にロックを奪われない）。保持者が旧IDでなければ何もしない。
		// 支出の更新自体は成立しているため、ロックの移動に失敗しても更新は失敗にせずログだけ残す。
		if rid, ok := updated.Reservation.ID(); ok {
			if _, err := u.reservations.ReplaceFulfillment(ctx, rid, closingDay.SettlementMonth(updated.Date), string(id), string(updated.ID)); err != nil {
				slog.Warn("予約の入力ロックの移動に失敗しました", "reservationId", rid, "from", id, "to", updated.ID, "error", err)
			}
		}
		if err := u.expenses.Delete(ctx, id); err != nil {
			return domain.Expense{}, fmt.Errorf("旧支出の削除に失敗しました: %w", err)
		}
	}
	return updated, nil
}

// ListByMonth は対象精算月の共有支出を日付降順で返す。
// 締め日設定に応じて精算期間（暦月をまたぐ場合がある）で集計する。
func (u *ExpenseUsecase) ListByMonth(ctx context.Context, month string) ([]domain.Expense, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return nil, err
	}
	list, err := expensesForSettlementMonth(ctx, u.expenses, ym, closingDay)
	if err != nil {
		return nil, fmt.Errorf("支出の取得に失敗しました: %w", err)
	}
	sort.Slice(list, func(i, j int) bool {
		if !list[i].Date.Equal(list[j].Date) {
			return list[i].Date.After(list[j].Date)
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

// Delete は共有支出を削除する。クライアントどちらのメンバーでも削除できる。
func (u *ExpenseUsecase) Delete(ctx context.Context, id domain.ExpenseID) error {
	if _, err := id.Month(); err != nil {
		return err
	}
	existing, err := u.expenses.FindByID(ctx, id)
	if err != nil {
		return err
	}
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return err
	}
	if err := ensureMonthNotSettled(ctx, u.snapshots, closingDay.SettlementMonth(existing.Date)); err != nil {
		return err
	}
	if err := u.expenses.Delete(ctx, id); err != nil {
		return fmt.Errorf("支出の削除に失敗しました: %w", err)
	}
	return nil
}
