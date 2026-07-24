package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// DirectTransferUsecase は立替精算（共有支出とは別の A→B 送金）に関するユースケース。
type DirectTransferUsecase struct {
	couple    domain.Couple
	transfers DirectTransferRepository
}

// NewDirectTransferUsecase は DirectTransferUsecase を生成する。
func NewDirectTransferUsecase(couple domain.Couple, transfers DirectTransferRepository) *DirectTransferUsecase {
	return &DirectTransferUsecase{couple: couple, transfers: transfers}
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

// Register は立替精算を登録する。
func (u *DirectTransferUsecase) Register(ctx context.Context, in RegisterDirectTransferInput) (domain.DirectTransfer, error) {
	dt, err := u.build(newIDSuffix(), in)
	if err != nil {
		return domain.DirectTransfer{}, err
	}
	if err := u.transfers.Save(ctx, dt); err != nil {
		return domain.DirectTransfer{}, fmt.Errorf("立替精算の保存に失敗しました: %w", err)
	}
	return dt, nil
}

// Update は既存の立替精算の内容を更新する（IDと継続/単発の別・対象月は維持）。
func (u *DirectTransferUsecase) Update(ctx context.Context, id domain.DirectTransferID, in RegisterDirectTransferInput) (domain.DirectTransfer, error) {
	existing, err := u.transfers.FindByID(ctx, id)
	if err != nil {
		return domain.DirectTransfer{}, err
	}
	to, ok := u.couple.Other(in.From)
	if !ok {
		return domain.DirectTransfer{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.From)
	}
	// 継続/単発の別と対象月は既存の値を維持する（変更するには削除して再登録する）。
	dt, err := domain.NewDirectTransfer(string(id), in.From, to.ID, domain.Money(in.AmountYen), in.Description, existing.Month)
	if err != nil {
		return domain.DirectTransfer{}, err
	}
	if err := u.transfers.Save(ctx, dt); err != nil {
		return domain.DirectTransfer{}, fmt.Errorf("立替精算の更新に失敗しました: %w", err)
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

// Delete は立替精算を削除する。
func (u *DirectTransferUsecase) Delete(ctx context.Context, id domain.DirectTransferID) error {
	if _, err := u.transfers.FindByID(ctx, id); err != nil {
		return err
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
