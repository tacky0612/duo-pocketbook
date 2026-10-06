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

// FindByIDConsistent はIDで支出を取得する（インメモリは常に強い整合性）。
func (r *ExpenseRepository) FindByIDConsistent(ctx context.Context, id domain.ExpenseID) (domain.Expense, error) {
	return r.FindByID(ctx, id)
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

// SalaryRepository は application.SalaryRepository のインメモリ実装。
type SalaryRepository struct {
	mu       sync.RWMutex
	salaries map[string]domain.Salary // key: month + memberID
}

// NewSalaryRepository は空の SalaryRepository を生成する。
func NewSalaryRepository() *SalaryRepository {
	return &SalaryRepository{salaries: map[string]domain.Salary{}}
}

func salaryKey(month domain.YearMonth, id domain.MemberID) string {
	return month.String() + "#" + string(id)
}

// Save は給与を保存（上書き）する。
func (r *SalaryRepository) Save(_ context.Context, salary domain.Salary) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.salaries[salaryKey(salary.Month, salary.MemberID)] = salary
	return nil
}

// FindByMonth は対象月の給与を返す。
func (r *SalaryRepository) FindByMonth(_ context.Context, month domain.YearMonth) ([]domain.Salary, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []domain.Salary
	for _, salary := range r.salaries {
		if salary.Month == month {
			list = append(list, salary)
		}
	}
	return list, nil
}

// Delete は対象月・メンバーの給与を削除する（存在しなくてもエラーにしない）。
func (r *SalaryRepository) Delete(_ context.Context, month domain.YearMonth, memberID domain.MemberID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.salaries, salaryKey(month, memberID))
	return nil
}

// IncomeRepository は application.IncomeRepository のインメモリ実装（追加収入・複数件）。
type IncomeRepository struct {
	mu    sync.RWMutex
	items map[domain.IncomeID]domain.Income
}

// NewIncomeRepository は空の IncomeRepository を生成する。
func NewIncomeRepository() *IncomeRepository {
	return &IncomeRepository{items: map[domain.IncomeID]domain.Income{}}
}

// Save は収入を保存する。
func (r *IncomeRepository) Save(_ context.Context, inc domain.Income) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[inc.ID] = inc
	return nil
}

// FindByID はIDで収入を取得する。
func (r *IncomeRepository) FindByID(_ context.Context, id domain.IncomeID) (domain.Income, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inc, ok := r.items[id]
	if !ok {
		return domain.Income{}, application.ErrNotFound
	}
	return inc, nil
}

// FindByIDConsistent はIDで収入を取得する（インメモリは常に強い整合性）。
func (r *IncomeRepository) FindByIDConsistent(ctx context.Context, id domain.IncomeID) (domain.Income, error) {
	return r.FindByID(ctx, id)
}

// FindRecurring は毎月継続の収入をすべて返す。
func (r *IncomeRepository) FindRecurring(_ context.Context) ([]domain.Income, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []domain.Income
	for _, inc := range r.items {
		if inc.IsRecurring() {
			list = append(list, inc)
		}
	}
	return list, nil
}

// FindByMonth は指定精算月の単発の収入を返す。
func (r *IncomeRepository) FindByMonth(_ context.Context, month domain.YearMonth) ([]domain.Income, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []domain.Income
	for _, inc := range r.items {
		if !inc.IsRecurring() && inc.Month == month {
			list = append(list, inc)
		}
	}
	return list, nil
}

// Delete は収入を削除する。
func (r *IncomeRepository) Delete(_ context.Context, id domain.IncomeID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return nil
}

// ReservationRepository は application.ReservationRepository のインメモリ実装（予約・入力ロック）。
type ReservationRepository struct {
	mu    sync.RWMutex
	items map[domain.ReservationID]domain.Reservation
	locks map[string]string // key: 精算月#予約ID → 入力ロックを持つ実データのID
}

// NewReservationRepository は空の ReservationRepository を生成する。
func NewReservationRepository() *ReservationRepository {
	return &ReservationRepository{
		items: map[domain.ReservationID]domain.Reservation{},
		locks: map[string]string{},
	}
}

// Save は予約の内容を保存する。「今月はなし」の記録は既存のものを維持する（単発なら消す）。
func (r *ReservationRepository) Save(_ context.Context, rsv domain.Reservation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rsv.SkippedMonths = nil
	if existing, ok := r.items[rsv.ID]; ok {
		rsv = rsv.RestoreSkippedMonths(existing.SkippedMonths)
	}
	r.items[rsv.ID] = rsv
	return nil
}

// AddSkip は予約の「今月はなし」に精算月を追加する。
func (r *ReservationRepository) AddSkip(_ context.Context, id domain.ReservationID, month domain.YearMonth) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rsv, ok := r.items[id]
	if !ok {
		return application.ErrNotFound
	}
	r.items[id] = rsv.RestoreSkippedMonths(append(append([]domain.YearMonth{}, rsv.SkippedMonths...), month))
	return nil
}

// RemoveSkip は予約の「今月はなし」から精算月を削除する。
func (r *ReservationRepository) RemoveSkip(_ context.Context, id domain.ReservationID, month domain.YearMonth) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rsv, ok := r.items[id]
	if !ok {
		return application.ErrNotFound
	}
	r.items[id] = rsv.Unskip(month)
	return nil
}

// FindByID はIDで予約を取得する。
func (r *ReservationRepository) FindByID(_ context.Context, id domain.ReservationID) (domain.Reservation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rsv, ok := r.items[id]
	if !ok {
		return domain.Reservation{}, application.ErrNotFound
	}
	return rsv, nil
}

// FindAll はすべての予約を返す。
func (r *ReservationRepository) FindAll(_ context.Context) ([]domain.Reservation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]domain.Reservation, 0, len(r.items))
	for _, rsv := range r.items {
		list = append(list, rsv)
	}
	return list, nil
}

// Delete は予約を削除する。
func (r *ReservationRepository) Delete(_ context.Context, id domain.ReservationID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return nil
}

func fulfillmentKey(id domain.ReservationID, month domain.YearMonth) string {
	return month.String() + "#" + string(id)
}

// AcquireFulfillment は入力ロックを条件付きで取得する。
func (r *ReservationRepository) AcquireFulfillment(_ context.Context, id domain.ReservationID, month domain.YearMonth, target string) (bool, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fulfillmentKey(id, month)
	if current, ok := r.locks[key]; ok {
		return false, current, nil
	}
	r.locks[key] = target
	return true, target, nil
}

// ReplaceFulfillment は入力ロックの保持者が stale のときだけ next に置き換える。
func (r *ReservationRepository) ReplaceFulfillment(_ context.Context, id domain.ReservationID, month domain.YearMonth, stale, next string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := fulfillmentKey(id, month)
	if r.locks[key] != stale {
		return false, nil
	}
	r.locks[key] = next
	return true, nil
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

// DirectTransferRepository は application.DirectTransferRepository のインメモリ実装。
type DirectTransferRepository struct {
	mu    sync.RWMutex
	items map[domain.DirectTransferID]domain.DirectTransfer
}

// NewDirectTransferRepository は空の DirectTransferRepository を生成する。
func NewDirectTransferRepository() *DirectTransferRepository {
	return &DirectTransferRepository{items: map[domain.DirectTransferID]domain.DirectTransfer{}}
}

// Save は立替精算を保存する。
func (r *DirectTransferRepository) Save(_ context.Context, dt domain.DirectTransfer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[dt.ID] = dt
	return nil
}

// FindByID はIDで立替精算を取得する。
func (r *DirectTransferRepository) FindByID(_ context.Context, id domain.DirectTransferID) (domain.DirectTransfer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	dt, ok := r.items[id]
	if !ok {
		return domain.DirectTransfer{}, application.ErrNotFound
	}
	return dt, nil
}

// FindRecurring は毎月継続の立替精算をすべて返す。
func (r *DirectTransferRepository) FindRecurring(_ context.Context) ([]domain.DirectTransfer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []domain.DirectTransfer
	for _, dt := range r.items {
		if dt.IsRecurring() {
			list = append(list, dt)
		}
	}
	return list, nil
}

// FindByMonth は指定精算月の単発の立替精算を返す。
func (r *DirectTransferRepository) FindByMonth(_ context.Context, month domain.YearMonth) ([]domain.DirectTransfer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []domain.DirectTransfer
	for _, dt := range r.items {
		if !dt.IsRecurring() && dt.Month == month {
			list = append(list, dt)
		}
	}
	return list, nil
}

// Delete は立替精算を削除する。
func (r *DirectTransferRepository) Delete(_ context.Context, id domain.DirectTransferID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return nil
}

// SettlementSnapshotRepository は application.SettlementSnapshotRepository のインメモリ実装。
type SettlementSnapshotRepository struct {
	mu        sync.RWMutex
	snapshots map[string]domain.SettlementSnapshot // key: 対象月
}

// NewSettlementSnapshotRepository は空の SettlementSnapshotRepository を生成する。
func NewSettlementSnapshotRepository() *SettlementSnapshotRepository {
	return &SettlementSnapshotRepository{snapshots: map[string]domain.SettlementSnapshot{}}
}

// Save はスナップショットを保存（上書き）する。
func (r *SettlementSnapshotRepository) Save(_ context.Context, s domain.SettlementSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshots[s.Month().String()] = s
	return nil
}

// Find は対象月のスナップショットを返す。存在しない場合は ok=false を返す。
func (r *SettlementSnapshotRepository) Find(_ context.Context, month domain.YearMonth) (domain.SettlementSnapshot, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.snapshots[month.String()]
	return s, ok, nil
}

// Delete は対象月のスナップショットを削除する。
func (r *SettlementSnapshotRepository) Delete(_ context.Context, month domain.YearMonth) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.snapshots, month.String())
	return nil
}

// SettingsRepository は application.SettingsRepository のインメモリ実装。
type SettingsRepository struct {
	mu            sync.RWMutex
	weight        domain.Weight
	set           bool
	profiles      map[domain.MemberID]application.MemberProfile
	closingDay    domain.ClosingDay
	closingDaySet bool
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

// GetClosingDay は設定済みの締め日を返す。
func (r *SettingsRepository) GetClosingDay(_ context.Context) (domain.ClosingDay, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.closingDay, r.closingDaySet, nil
}

// SaveClosingDay は締め日を保存する。
func (r *SettingsRepository) SaveClosingDay(_ context.Context, day domain.ClosingDay) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closingDay = day
	r.closingDaySet = true
	return nil
}

// AccountRepository は application.AccountRepository のインメモリ実装。
type AccountRepository struct {
	mu       sync.RWMutex
	accounts map[domain.MemberID]application.Account
}

// NewAccountRepository は空の AccountRepository を生成する。
func NewAccountRepository() *AccountRepository {
	return &AccountRepository{accounts: map[domain.MemberID]application.Account{}}
}

// List は全アカウントを返す。
func (r *AccountRepository) List(_ context.Context) ([]application.Account, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]application.Account, 0, len(r.accounts))
	for _, a := range r.accounts {
		out = append(out, a)
	}
	return out, nil
}

// Save はアカウントを保存（upsert）する。
func (r *AccountRepository) Save(_ context.Context, a application.Account) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accounts[a.ID] = a
	return nil
}
