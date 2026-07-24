package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// IncomeUsecase は給与とは別の追加収入（内容付き・単発/継続）に関するユースケース。
type IncomeUsecase struct {
	couple  domain.Couple
	incomes IncomeRepository
}

// NewIncomeUsecase は IncomeUsecase を生成する。
func NewIncomeUsecase(couple domain.Couple, incomes IncomeRepository) *IncomeUsecase {
	return &IncomeUsecase{couple: couple, incomes: incomes}
}

// RegisterIncomeInput は収入登録の入力。
// Month が空文字なら毎月継続、"YYYY-MM" ならその精算月のみの単発として扱う。
type RegisterIncomeInput struct {
	MemberID    domain.MemberID
	AmountYen   int64
	Description string
	Month       string
}

// build は指定サフィックスの ID で Income を組み立てる。
func (u *IncomeUsecase) build(suffix string, in RegisterIncomeInput) (domain.Income, error) {
	if !u.couple.Contains(in.MemberID) {
		return domain.Income{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.MemberID)
	}
	var (
		month domain.YearMonth
		id    domain.IncomeID
	)
	if in.Month == "" {
		id = domain.NewRecurringIncomeID(suffix)
	} else {
		ym, err := domain.ParseYearMonth(in.Month)
		if err != nil {
			return domain.Income{}, err
		}
		month = ym
		id = domain.NewOneOffIncomeID(ym, suffix)
	}
	return domain.NewIncome(string(id), in.MemberID, domain.Money(in.AmountYen), in.Description, month)
}

// Register は収入を登録する。
func (u *IncomeUsecase) Register(ctx context.Context, in RegisterIncomeInput) (domain.Income, error) {
	inc, err := u.build(newIDSuffix(), in)
	if err != nil {
		return domain.Income{}, err
	}
	if err := u.incomes.Save(ctx, inc); err != nil {
		return domain.Income{}, fmt.Errorf("収入の保存に失敗しました: %w", err)
	}
	return inc, nil
}

// Update は既存の収入の内容を更新する（IDと継続/単発の別・対象月は維持）。
func (u *IncomeUsecase) Update(ctx context.Context, id domain.IncomeID, in RegisterIncomeInput) (domain.Income, error) {
	existing, err := u.incomes.FindByID(ctx, id)
	if err != nil {
		return domain.Income{}, err
	}
	if !u.couple.Contains(in.MemberID) {
		return domain.Income{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, in.MemberID)
	}
	// 継続/単発の別と対象月は既存の値を維持する（変更するには削除して再登録する）。
	inc, err := domain.NewIncome(string(id), in.MemberID, domain.Money(in.AmountYen), in.Description, existing.Month)
	if err != nil {
		return domain.Income{}, err
	}
	if err := u.incomes.Save(ctx, inc); err != nil {
		return domain.Income{}, fmt.Errorf("収入の更新に失敗しました: %w", err)
	}
	return inc, nil
}

// ListForMonth は指定精算月に適用される収入（毎月継続分＋当月単発分）を返す。
// 継続を先に、単発を後に並べ、各グループ内は内容の昇順で返す。
func (u *IncomeUsecase) ListForMonth(ctx context.Context, month string) ([]domain.Income, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	recurring, err := u.incomes.FindRecurring(ctx)
	if err != nil {
		return nil, fmt.Errorf("収入の取得に失敗しました: %w", err)
	}
	oneOff, err := u.incomes.FindByMonth(ctx, ym)
	if err != nil {
		return nil, fmt.Errorf("収入の取得に失敗しました: %w", err)
	}
	sortIncomesByDescription(recurring)
	sortIncomesByDescription(oneOff)
	return append(recurring, oneOff...), nil
}

// Delete は収入を削除する。
func (u *IncomeUsecase) Delete(ctx context.Context, id domain.IncomeID) error {
	if _, err := u.incomes.FindByID(ctx, id); err != nil {
		return err
	}
	if err := u.incomes.Delete(ctx, id); err != nil {
		return fmt.Errorf("収入の削除に失敗しました: %w", err)
	}
	return nil
}

func sortIncomesByDescription(list []domain.Income) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Description < list[j].Description
	})
}
