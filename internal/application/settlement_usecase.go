package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// maxHistoryMonths は履歴取得で一度に走査する最大月数。
const maxHistoryMonths = 120

// SettlementHistoryEntry は履歴の1か月分（精算結果＋精算済みフラグ）。
type SettlementHistoryEntry struct {
	Settlement *domain.Settlement
	Settled    bool
}

// SettlementUsecase は月次収入と精算に関するユースケース。
type SettlementUsecase struct {
	couple    domain.Couple
	expenses  ExpenseRepository
	incomes   IncomeRepository
	recurring RecurringExpenseRepository
	settings  SettingsRepository
	status    SettlementStatusRepository
}

// NewSettlementUsecase は SettlementUsecase を生成する。
func NewSettlementUsecase(couple domain.Couple, expenses ExpenseRepository, incomes IncomeRepository, recurring RecurringExpenseRepository, settings SettingsRepository, status SettlementStatusRepository) *SettlementUsecase {
	return &SettlementUsecase{couple: couple, expenses: expenses, incomes: incomes, recurring: recurring, settings: settings, status: status}
}

// IsSettled は対象月が精算済みかを返す。
func (u *SettlementUsecase) IsSettled(ctx context.Context, month string) (bool, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return false, err
	}
	settled, err := u.status.IsSettled(ctx, ym)
	if err != nil {
		return false, fmt.Errorf("精算ステータスの取得に失敗しました: %w", err)
	}
	return settled, nil
}

// SetSettled は対象月の精算済みフラグを更新する。
func (u *SettlementUsecase) SetSettled(ctx context.Context, month string, settled bool) (bool, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return false, err
	}
	if err := u.status.SetSettled(ctx, ym, settled); err != nil {
		return false, fmt.Errorf("精算ステータスの保存に失敗しました: %w", err)
	}
	return settled, nil
}

// InputIncome は対象月のメンバーの収入を入力（上書き）する。
func (u *SettlementUsecase) InputIncome(ctx context.Context, month string, memberID domain.MemberID, amountYen int64) (domain.MonthlyIncome, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return domain.MonthlyIncome{}, err
	}
	if !u.couple.Contains(memberID) {
		return domain.MonthlyIncome{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, memberID)
	}
	income, err := domain.NewMonthlyIncome(ym, memberID, domain.Money(amountYen))
	if err != nil {
		return domain.MonthlyIncome{}, err
	}
	if err := u.incomes.Save(ctx, income); err != nil {
		return domain.MonthlyIncome{}, fmt.Errorf("収入の保存に失敗しました: %w", err)
	}
	return income, nil
}

// GetIncomes は対象月の入力済み収入を返す。
func (u *SettlementUsecase) GetIncomes(ctx context.Context, month string) ([]domain.MonthlyIncome, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	list, err := u.incomes.FindByMonth(ctx, ym)
	if err != nil {
		return nil, fmt.Errorf("収入の取得に失敗しました: %w", err)
	}
	return list, nil
}

// GetSettlement は対象月の精算結果を計算して返す。
// 両メンバーの収入が入力されていない場合は domain.ErrIncomeNotReady を返す。
func (u *SettlementUsecase) GetSettlement(ctx context.Context, month string) (*domain.Settlement, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	incomes, err := u.incomes.FindByMonth(ctx, ym)
	if err != nil {
		return nil, fmt.Errorf("収入の取得に失敗しました: %w", err)
	}
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return nil, err
	}
	expenses, err := expensesForSettlementMonth(ctx, u.expenses, ym, closingDay)
	if err != nil {
		return nil, fmt.Errorf("支出の取得に失敗しました: %w", err)
	}
	// 固定費を対象月の共有支出として実体化し、通常の支出とあわせて精算する。
	recurring, err := u.recurring.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("固定費の取得に失敗しました: %w", err)
	}
	for _, r := range recurring {
		expenses = append(expenses, r.AsExpenseFor(ym))
	}
	weight, err := currentWeight(ctx, u.settings, u.couple)
	if err != nil {
		return nil, err
	}
	// 精算結果に表示する名前へ、上書き済みの表示名を反映する。
	couple, err := effectiveCouple(ctx, u.settings, u.couple)
	if err != nil {
		return nil, err
	}
	return domain.CalculateSettlement(domain.SettlementInput{
		Month:      ym,
		Couple:     couple,
		Incomes:    incomes,
		Expenses:   expenses,
		Weight:     weight,
		ClosingDay: closingDay,
	})
}

// History は from〜to（両端含む・YYYY-MM）の各月について精算結果と精算済みフラグを、
// 新しい月から順に返す。両メンバーの収入が未入力の月はスキップする。
func (u *SettlementUsecase) History(ctx context.Context, from, to string) ([]SettlementHistoryEntry, error) {
	fromYM, err := domain.ParseYearMonth(from)
	if err != nil {
		return nil, err
	}
	toYM, err := domain.ParseYearMonth(to)
	if err != nil {
		return nil, err
	}
	fromIdx := monthIndex(fromYM)
	toIdx := monthIndex(toYM)
	if fromIdx > toIdx {
		return nil, fmt.Errorf("%w: fromはto以前の月を指定してください", domain.ErrValidation)
	}
	if toIdx-fromIdx+1 > maxHistoryMonths {
		return nil, fmt.Errorf("%w: 一度に取得できるのは%dか月までです", domain.ErrValidation, maxHistoryMonths)
	}

	var entries []SettlementHistoryEntry
	for cur := toYM; monthIndex(cur) >= fromIdx; cur = prevMonth(cur) {
		s, err := u.GetSettlement(ctx, cur.String())
		if errors.Is(err, domain.ErrIncomeNotReady) {
			continue // 収入未入力の月は履歴に含めない
		}
		if err != nil {
			return nil, err
		}
		settled, err := u.status.IsSettled(ctx, cur)
		if err != nil {
			return nil, fmt.Errorf("精算ステータスの取得に失敗しました: %w", err)
		}
		entries = append(entries, SettlementHistoryEntry{Settlement: s, Settled: settled})
	}
	return entries, nil
}

// monthIndex は年月を「年×12＋月」の連番に変換して比較・差分計算に使う。
func monthIndex(ym domain.YearMonth) int {
	return ym.Year()*12 + int(ym.Month()) - 1
}

// prevMonth は1か月前の YearMonth を返す。
func prevMonth(ym domain.YearMonth) domain.YearMonth {
	return ym.Prev()
}

// expensesForSettlementMonth は精算月 ym に計上すべき支出を返す。
//
// 締め日=1（暦月どおり）なら該当月パーティションをそのまま返す。締め日 D>=2 のときは、
// 精算期間が暦月をまたぐ（ym の前月の締め日以降 〜 ym の締め日前日）ため、暦月 ym-1 と ym の
// 2パーティションを取得し、各支出の精算月が ym と一致するものだけを返す。
// 支出は暦月（支出日の年月）をキーに保存されるため、締め日を変更しても保存先は変わらない。
func expensesForSettlementMonth(ctx context.Context, repo ExpenseRepository, ym domain.YearMonth, cd domain.ClosingDay) ([]domain.Expense, error) {
	if cd <= domain.DefaultClosingDay {
		return repo.FindByMonth(ctx, ym)
	}
	var out []domain.Expense
	for _, cal := range [2]domain.YearMonth{ym.Prev(), ym} {
		list, err := repo.FindByMonth(ctx, cal)
		if err != nil {
			return nil, err
		}
		for _, e := range list {
			if cd.SettlementMonth(e.Date) == ym {
				out = append(out, e)
			}
		}
	}
	return out, nil
}
