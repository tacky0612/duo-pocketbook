package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// DirectTransferUsecase は立替精算（共有支出とは別の A→B 送金）に関するユースケース。
type DirectTransferUsecase struct {
	couple    domain.Couple
	transfers DirectTransferRepository
	snapshots SettlementSnapshotRepository
}

// NewDirectTransferUsecase は DirectTransferUsecase を生成する。
func NewDirectTransferUsecase(couple domain.Couple, transfers DirectTransferRepository, snapshots SettlementSnapshotRepository) *DirectTransferUsecase {
	return &DirectTransferUsecase{couple: couple, transfers: transfers, snapshots: snapshots}
}

// RegisterDirectTransferInput は立替精算登録の入力。
// Month が空文字なら毎月継続、"YYYY-MM" ならその精算月のみの単発として扱う。
type RegisterDirectTransferInput struct {
	From        domain.MemberID
	AmountYen   int64
	Description string
	Month       string
}

// build は入力から送金先（送金元でない方）を導出し、指定サフィックスの ID で DirectTransfer を組み立てる。
func (u *DirectTransferUsecase) build(suffix string, in RegisterDirectTransferInput) (domain.DirectTransfer, error) {
	to, ok := u.couple.Other(in.From)
	if !ok {
		return domain.DirectTransfer{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.From)
	}
	var (
		month domain.YearMonth
		id    domain.DirectTransferID
	)
	if in.Month == "" {
		id = domain.NewRecurringDirectTransferID(suffix)
	} else {
		ym, err := domain.ParseYearMonth(in.Month)
		if err != nil {
			return domain.DirectTransfer{}, err
		}
		month = ym
		id = domain.NewOneOffDirectTransferID(ym, suffix)
	}
	return domain.NewDirectTransfer(string(id), in.From, to.ID, domain.Money(in.AmountYen), in.Description, month)
}

// Register は立替精算を登録する。単発（特定月）の登録先が確定済みの月なら拒否する。
func (u *DirectTransferUsecase) Register(ctx context.Context, in RegisterDirectTransferInput) (domain.DirectTransfer, error) {
	if in.Month != "" {
		ym, err := domain.ParseYearMonth(in.Month)
		if err != nil {
			return domain.DirectTransfer{}, err
		}
		if err := ensureMonthNotSettled(ctx, u.snapshots, ym); err != nil {
			return domain.DirectTransfer{}, err
		}
	}
	dt, err := u.build(newIDSuffix(), in)
	if err != nil {
		return domain.DirectTransfer{}, err
	}
	if err := u.transfers.Save(ctx, dt); err != nil {
		return domain.DirectTransfer{}, fmt.Errorf("立替精算の保存に失敗しました: %w", err)
	}
	return dt, nil
}

// Update は既存の立替精算の内容を更新する。
// 頻度（継続/単発の別と対象月）も in.Month で変更でき、頻度が変わるとIDを移し替える
// （新しいIDで保存し、旧レコードを削除する）。in.Month が空文字なら毎月継続、
// "YYYY-MM" ならその精算月のみの単発。
func (u *DirectTransferUsecase) Update(ctx context.Context, id domain.DirectTransferID, in RegisterDirectTransferInput) (domain.DirectTransfer, error) {
	existing, err := u.transfers.FindByID(ctx, id)
	if err != nil {
		return domain.DirectTransfer{}, err
	}
	to, ok := u.couple.Other(in.From)
	if !ok {
		return domain.DirectTransfer{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.From)
	}
	// 変更後の対象月（頻度）を決める。
	var month domain.YearMonth
	if in.Month != "" {
		ym, err := domain.ParseYearMonth(in.Month)
		if err != nil {
			return domain.DirectTransfer{}, err
		}
		month = ym
	}
	// 変更前・変更後（単発の場合）の月が確定済みなら編集を拒否する
	// （頻度変更で確定済みの月へ移す／から出すのも不可）。継続（月なし）は対象外。
	if !existing.Month.IsZero() {
		if err := ensureMonthNotSettled(ctx, u.snapshots, existing.Month); err != nil {
			return domain.DirectTransfer{}, err
		}
	}
	if !month.IsZero() {
		if err := ensureMonthNotSettled(ctx, u.snapshots, month); err != nil {
			return domain.DirectTransfer{}, err
		}
	}
	// 既存IDのサフィックスを引き継ぎ、頻度に応じたIDを組む。頻度が変わるとIDが変化する。
	_, suffix, _ := strings.Cut(string(id), "_")
	var newID domain.DirectTransferID
	if month.IsZero() {
		newID = domain.NewRecurringDirectTransferID(suffix)
	} else {
		newID = domain.NewOneOffDirectTransferID(month, suffix)
	}
	dt, err := domain.NewDirectTransfer(string(newID), in.From, to.ID, domain.Money(in.AmountYen), in.Description, month)
	if err != nil {
		return domain.DirectTransfer{}, err
	}
	if err := u.transfers.Save(ctx, dt); err != nil {
		return domain.DirectTransfer{}, fmt.Errorf("立替精算の更新に失敗しました: %w", err)
	}
	// 頻度変更などでIDが変わった場合は旧レコードを削除する。
	if dt.ID != id {
		if err := u.transfers.Delete(ctx, id); err != nil {
			return domain.DirectTransfer{}, fmt.Errorf("旧立替精算の削除に失敗しました: %w", err)
		}
	}
	return dt, nil
}

// ListForMonth は指定精算月に適用される立替精算（毎月継続分＋当月単発分）を返す。
// 継続を先に、単発を後に並べ、各グループ内は内容の昇順で返す。
func (u *DirectTransferUsecase) ListForMonth(ctx context.Context, month string) ([]domain.DirectTransfer, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	recurring, err := u.transfers.FindRecurring(ctx)
	if err != nil {
		return nil, fmt.Errorf("立替精算の取得に失敗しました: %w", err)
	}
	oneOff, err := u.transfers.FindByMonth(ctx, ym)
	if err != nil {
		return nil, fmt.Errorf("立替精算の取得に失敗しました: %w", err)
	}
	sortByDescription(recurring)
	sortByDescription(oneOff)
	return append(recurring, oneOff...), nil
}

// Delete は立替精算を削除する。単発（特定月）でその月が確定済みなら拒否する。継続は対象外。
func (u *DirectTransferUsecase) Delete(ctx context.Context, id domain.DirectTransferID) error {
	existing, err := u.transfers.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !existing.Month.IsZero() {
		if err := ensureMonthNotSettled(ctx, u.snapshots, existing.Month); err != nil {
			return err
		}
	}
	if err := u.transfers.Delete(ctx, id); err != nil {
		return fmt.Errorf("立替精算の削除に失敗しました: %w", err)
	}
	return nil
}

func sortByDescription(list []domain.DirectTransfer) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Description < list[j].Description
	})
}
