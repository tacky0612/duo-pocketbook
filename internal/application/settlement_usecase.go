package application

import (
	"context"
	"fmt"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// maxHistoryMonths は履歴取得で一度に走査する最大月数。
const maxHistoryMonths = 120

// SettlementUsecase は給与・収入と精算に関するユースケース。
type SettlementUsecase struct {
	couple    domain.Couple
	expenses  ExpenseRepository
	salaries  SalaryRepository
	incomes   IncomeRepository
	recurring RecurringExpenseRepository
	transfers DirectTransferRepository
	settings  SettingsRepository
	snapshots SettlementSnapshotRepository
	now       func() time.Time
}

// NewSettlementUsecase は SettlementUsecase を生成する。now が nil の場合は time.Now を使う。
func NewSettlementUsecase(couple domain.Couple, expenses ExpenseRepository, salaries SalaryRepository, incomes IncomeRepository, recurring RecurringExpenseRepository, transfers DirectTransferRepository, settings SettingsRepository, snapshots SettlementSnapshotRepository, now func() time.Time) *SettlementUsecase {
	if now == nil {
		now = time.Now
	}
	return &SettlementUsecase{couple: couple, expenses: expenses, salaries: salaries, incomes: incomes, recurring: recurring, transfers: transfers, settings: settings, snapshots: snapshots, now: now}
}

// IsSettled は対象月が精算済みか（スナップショットが存在するか）を返す。
func (u *SettlementUsecase) IsSettled(ctx context.Context, month string) (bool, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return false, err
	}
	_, ok, err := u.snapshots.Find(ctx, ym)
	if err != nil {
		return false, fmt.Errorf("精算スナップショットの取得に失敗しました: %w", err)
	}
	return ok, nil
}

// SetSettled は対象月の精算完了状態を更新する。
//
// settled=true のときは、その時点の精算内容をスナップショットとして保存する。
// 両メンバーの給与が未入力で精算を計算できない場合は domain.ErrIncomeNotReady を返す。
// settled=false のときはスナップショットを削除する（精算済みを取り消す）。
func (u *SettlementUsecase) SetSettled(ctx context.Context, month string, settled bool) (bool, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return false, err
	}
	if !settled {
		if err := u.snapshots.Delete(ctx, ym); err != nil {
			return false, fmt.Errorf("精算スナップショットの削除に失敗しました: %w", err)
		}
		return false, nil
	}
	snapshot, err := u.buildSnapshot(ctx, ym)
	if err != nil {
		return false, err
	}
	if err := u.snapshots.Save(ctx, snapshot); err != nil {
		return false, fmt.Errorf("精算スナップショットの保存に失敗しました: %w", err)
	}
	return true, nil
}

// buildSnapshot は対象月の精算内容を計算し、完了時点の記録としてスナップショットを組み立てる。
func (u *SettlementUsecase) buildSnapshot(ctx context.Context, ym domain.YearMonth) (domain.SettlementSnapshot, error) {
	settlement, expenseItems, directItems, err := u.computeSettlement(ctx, ym)
	if err != nil {
		return domain.SettlementSnapshot{}, err
	}
	return domain.SettlementSnapshot{
		Settlement:      *settlement,
		SettledAt:       u.now(),
		Expenses:        expenseItems,
		DirectTransfers: directItems,
	}, nil
}

// InputSalary は対象月のメンバーの給与を入力（上書き）する。
func (u *SettlementUsecase) InputSalary(ctx context.Context, month string, memberID domain.MemberID, amountYen int64) (domain.Salary, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return domain.Salary{}, err
	}
	if !u.couple.Contains(memberID) {
		return domain.Salary{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, memberID)
	}
	if err := ensureMonthNotSettled(ctx, u.snapshots, ym); err != nil {
		return domain.Salary{}, err
	}
	salary, err := domain.NewSalary(ym, memberID, domain.Money(amountYen))
	if err != nil {
		return domain.Salary{}, err
	}
	if err := u.salaries.Save(ctx, salary); err != nil {
		return domain.Salary{}, fmt.Errorf("給与の保存に失敗しました: %w", err)
	}
	return salary, nil
}

// GetSalaries は対象月の入力済み給与を返す。
func (u *SettlementUsecase) GetSalaries(ctx context.Context, month string) ([]domain.Salary, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	list, err := u.salaries.FindByMonth(ctx, ym)
	if err != nil {
		return nil, fmt.Errorf("給与の取得に失敗しました: %w", err)
	}
	return list, nil
}

// GetSettlement は対象月の精算結果を計算して返す。
// 両メンバーの給与が入力されていない場合は domain.ErrIncomeNotReady を返す。
func (u *SettlementUsecase) GetSettlement(ctx context.Context, month string) (*domain.Settlement, error) {
	ym, err := domain.ParseYearMonth(month)
	if err != nil {
		return nil, err
	}
	s, _, _, err := u.computeSettlement(ctx, ym)
	return s, err
}

// computeSettlement は対象月の精算に必要な入力を集めて精算結果を計算し、あわせて
// スナップショット用の共有支出・立替精算の明細を返す。
// 両メンバーの給与が入力されていない場合は domain.ErrIncomeNotReady を返す。
func (u *SettlementUsecase) computeSettlement(ctx context.Context, ym domain.YearMonth) (*domain.Settlement, []domain.SettlementExpenseItem, []domain.SettlementDirectTransferItem, error) {
	salaries, err := u.salaries.FindByMonth(ctx, ym)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("給与の取得に失敗しました: %w", err)
	}
	// 追加収入（毎月継続分＋当月単発分）を集める。給与と合算して各メンバーの収入とする。
	incomeRecurring, err := u.incomes.FindRecurring(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("収入の取得に失敗しました: %w", err)
	}
	incomeOneOff, err := u.incomes.FindByMonth(ctx, ym)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("収入の取得に失敗しました: %w", err)
	}
	incomes := append(incomeRecurring, incomeOneOff...)
	closingDay, err := currentClosingDay(ctx, u.settings)
	if err != nil {
		return nil, nil, nil, err
	}
	oneOffExpenses, err := expensesForSettlementMonth(ctx, u.expenses, ym, closingDay)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("支出の取得に失敗しました: %w", err)
	}
	// 固定費を対象月の共有支出として実体化し、通常の支出とあわせて精算する。
	recurring, err := u.recurring.FindAll(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("固定費の取得に失敗しました: %w", err)
	}
	expenses := append([]domain.Expense{}, oneOffExpenses...)
	for _, r := range recurring {
		expenses = append(expenses, r.AsExpenseFor(ym))
	}
	// 立替精算（毎月継続分＋当月単発分）を集める。比重按分には含めず振込額へ加算する。
	directRecurring, err := u.transfers.FindRecurring(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("立替精算の取得に失敗しました: %w", err)
	}
	directOneOff, err := u.transfers.FindByMonth(ctx, ym)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("立替精算の取得に失敗しました: %w", err)
	}
	directTransfers := append(directRecurring, directOneOff...)
	weight, err := currentWeight(ctx, u.settings, u.couple)
	if err != nil {
		return nil, nil, nil, err
	}
	// 精算結果に表示する名前へ、上書き済みの表示名を反映する。
	couple, err := effectiveCouple(ctx, u.settings, u.couple)
	if err != nil {
		return nil, nil, nil, err
	}
	settlement, err := domain.CalculateSettlement(domain.SettlementInput{
		Month:           ym,
		Couple:          couple,
		Salaries:        salaries,
		Incomes:         incomes,
		Expenses:        expenses,
		DirectTransfers: directTransfers,
		Weight:          weight,
		ClosingDay:      closingDay,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return settlement, expenseItems(oneOffExpenses, recurring), directTransferItems(directTransfers), nil
}

// expenseItems は精算スナップショット用に共有支出の明細を組み立てる。
// 通常の支出（Recurring=false）に続けて、固定費を Recurring=true として並べる。
func expenseItems(oneOff []domain.Expense, recurring []domain.RecurringExpense) []domain.SettlementExpenseItem {
	items := make([]domain.SettlementExpenseItem, 0, len(oneOff)+len(recurring))
	for _, e := range oneOff {
		items = append(items, domain.SettlementExpenseItem{
			PaidBy:      e.PaidBy,
			Amount:      e.Amount,
			Description: e.Description,
			Date:        e.Date.Format("2006-01-02"),
			Recurring:   false,
		})
	}
	for _, r := range recurring {
		items = append(items, domain.SettlementExpenseItem{
			PaidBy:      r.PaidBy,
			Amount:      r.Amount,
			Description: r.Description,
			Date:        "",
			Recurring:   true,
		})
	}
	return items
}

// directTransferItems は精算スナップショット用に立替精算の明細を組み立てる。
func directTransferItems(transfers []domain.DirectTransfer) []domain.SettlementDirectTransferItem {
	items := make([]domain.SettlementDirectTransferItem, 0, len(transfers))
	for _, dt := range transfers {
		items = append(items, domain.SettlementDirectTransferItem{
			From:        dt.From,
			To:          dt.To,
			Amount:      dt.Amount,
			Description: dt.Description,
			Recurring:   dt.IsRecurring(),
		})
	}
	return items
}

// History は from〜to（両端含む・YYYY-MM）の各月について、保存済みの精算スナップショットを
// 新しい月から順に返す。スナップショットの無い月（未精算の月）はスキップする。
func (u *SettlementUsecase) History(ctx context.Context, from, to string) ([]domain.SettlementSnapshot, error) {
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

	var snapshots []domain.SettlementSnapshot
	for cur := toYM; monthIndex(cur) >= fromIdx; cur = prevMonth(cur) {
		snapshot, ok, err := u.snapshots.Find(ctx, cur)
		if err != nil {
			return nil, fmt.Errorf("精算スナップショットの取得に失敗しました: %w", err)
		}
		if !ok {
			continue // 未精算の月は履歴に含めない
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
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
