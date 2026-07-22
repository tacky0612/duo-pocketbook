// Package memory はリポジトリのインメモリ実装を提供する。
// ユニットテストおよび永続化不要なローカル起動で利用する。
package memory

import (
	"context"
	"sync"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ExpenseRepository は application.ExpenseRepository のインメモリ実装。
type ExpenseRepository struct {
	mu       sync.RWMutex
	expenses map[domain.ExpenseID]domain.Expense
}

// NewExpenseRepository は空の ExpenseRepository を生成する。
func NewExpenseRepository() *ExpenseRepository {
	return &ExpenseRepository{expenses: map[domain.ExpenseID]domain.Expense{}}
}

// Save は支出を保存する。
func (r *ExpenseRepository) Save(_ context.Context, e domain.Expense) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expenses[e.ID] = e
	return nil
}

// FindByID はIDで支出を取得する。
func (r *ExpenseRepository) FindByID(_ context.Context, id domain.ExpenseID) (domain.Expense, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.expenses[id]
	if !ok {
		return domain.Expense{}, application.ErrNotFound
	}
	return e, nil
}

// FindByMonth は対象月の支出を返す。
func (r *ExpenseRepository) FindByMonth(_ context.Context, month domain.YearMonth) ([]domain.Expense, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []domain.Expense
	for _, e := range r.expenses {
		if e.Month() == month {
			list = append(list, e)
		}
	}
	return list, nil
}

// Delete は支出を削除する。
func (r *ExpenseRepository) Delete(_ context.Context, id domain.ExpenseID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.expenses, id)
	return nil
}

// IncomeRepository は application.IncomeRepository のインメモリ実装。
type IncomeRepository struct {
	mu      sync.RWMutex
	incomes map[string]domain.MonthlyIncome // key: month + memberID
}

// NewIncomeRepository は空の IncomeRepository を生成する。
func NewIncomeRepository() *IncomeRepository {
	return &IncomeRepository{incomes: map[string]domain.MonthlyIncome{}}
}

func incomeKey(month domain.YearMonth, id domain.MemberID) string {
	return month.String() + "#" + string(id)
}

// Save は収入を保存（上書き）する。
func (r *IncomeRepository) Save(_ context.Context, income domain.MonthlyIncome) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.incomes[incomeKey(income.Month, income.MemberID)] = income
	return nil
}

// FindByMonth は対象月の収入を返す。
func (r *IncomeRepository) FindByMonth(_ context.Context, month domain.YearMonth) ([]domain.MonthlyIncome, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []domain.MonthlyIncome
	for _, income := range r.incomes {
		if income.Month == month {
			list = append(list, income)
		}
	}
	return list, nil
}

// RecurringExpenseRepository は application.RecurringExpenseRepository のインメモリ実装。
type RecurringExpenseRepository struct {
	mu    sync.RWMutex
	items map[domain.RecurringExpenseID]domain.RecurringExpense
}

// NewRecurringExpenseRepository は空の RecurringExpenseRepository を生成する。
func NewRecurringExpenseRepository() *RecurringExpenseRepository {
	return &RecurringExpenseRepository{items: map[domain.RecurringExpenseID]domain.RecurringExpense{}}
}

// Save は固定費を保存する。
func (r *RecurringExpenseRepository) Save(_ context.Context, e domain.RecurringExpense) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[e.ID] = e
	return nil
}

// FindByID はIDで固定費を取得する。
func (r *RecurringExpenseRepository) FindByID(_ context.Context, id domain.RecurringExpenseID) (domain.RecurringExpense, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.items[id]
	if !ok {
		return domain.RecurringExpense{}, application.ErrNotFound
	}
	return e, nil
}

// FindAll は全固定費を返す。
func (r *RecurringExpenseRepository) FindAll(_ context.Context) ([]domain.RecurringExpense, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.RecurringExpense, 0, len(r.items))
	for _, e := range r.items {
		list = append(list, e)
	}
	return list, nil
}

// Delete は固定費を削除する。
func (r *RecurringExpenseRepository) Delete(_ context.Context, id domain.RecurringExpenseID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return nil
}

// SettlementStatusRepository は application.SettlementStatusRepository のインメモリ実装。
type SettlementStatusRepository struct {
	mu      sync.RWMutex
	settled map[string]bool // key: 対象月
}

// NewSettlementStatusRepository は空の SettlementStatusRepository を生成する。
func NewSettlementStatusRepository() *SettlementStatusRepository {
	return &SettlementStatusRepository{settled: map[string]bool{}}
}

// IsSettled は対象月が精算済みかを返す。
func (r *SettlementStatusRepository) IsSettled(_ context.Context, month domain.YearMonth) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.settled[month.String()], nil
}

// SetSettled は精算済みフラグを保存する。
func (r *SettlementStatusRepository) SetSettled(_ context.Context, month domain.YearMonth, settled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settled[month.String()] = settled
	return nil
}

// SettingsRepository は application.SettingsRepository のインメモリ実装。
type SettingsRepository struct {
	mu       sync.RWMutex
	weight   domain.Weight
	set      bool
	profiles map[domain.MemberID]application.MemberProfile
}

// NewSettingsRepository は空の SettingsRepository を生成する。
func NewSettingsRepository() *SettingsRepository {
	return &SettingsRepository{profiles: map[domain.MemberID]application.MemberProfile{}}
}

// GetMemberProfiles はプロフィールの上書き設定を返す。
func (r *SettingsRepository) GetMemberProfiles(_ context.Context) (map[domain.MemberID]application.MemberProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[domain.MemberID]application.MemberProfile, len(r.profiles))
	for id, p := range r.profiles {
		out[id] = p
	}
	return out, nil
}

// SaveMemberName は表示名を保存する（カラーは維持）。
func (r *SettingsRepository) SaveMemberName(_ context.Context, id domain.MemberID, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.profiles[id]
	p.Name = name
	r.profiles[id] = p
	return nil
}

// SaveMemberColor はカラーを保存する（表示名は維持）。
func (r *SettingsRepository) SaveMemberColor(_ context.Context, id domain.MemberID, color string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.profiles[id]
	p.Color = color
	r.profiles[id] = p
	return nil
}

// GetWeight は設定済みの比重を返す。
func (r *SettingsRepository) GetWeight(_ context.Context) (domain.Weight, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.weight, r.set, nil
}

// SaveWeight は比重を保存する。
func (r *SettingsRepository) SaveWeight(_ context.Context, weight domain.Weight) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.weight = weight
	r.set = true
	return nil
}
