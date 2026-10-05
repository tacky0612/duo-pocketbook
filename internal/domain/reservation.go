package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ReservationKind は予約の種別（支出の予約か収入の予約か）。
type ReservationKind string

const (
	// ReservationKindExpense は支出の予約。入力すると共有支出として登録される。
	ReservationKindExpense ReservationKind = "expense"
	// ReservationKindIncome は収入の予約。入力すると追加収入（単発）として登録される。
	ReservationKindIncome ReservationKind = "income"
)

// ParseReservationKind は文字列から予約の種別を解釈する。
func ParseReservationKind(s string) (ReservationKind, error) {
	switch k := ReservationKind(s); k {
	case ReservationKindExpense, ReservationKindIncome:
		return k, nil
	default:
		return "", fmt.Errorf("%w: 予約の種別は expense か income で指定してください: %q", ErrValidation, s)
	}
}

// ReservationID は予約の識別子（"rsv_<suffix>" 形式）。
//
// 頻度（毎月/単発）や対象月はIDに含めず属性として持つ。頻度を変更してもIDが変わらないため、
// 予約に紐づく支出・収入（ReservationRef）や入力ロックを付け替える必要がない。
type ReservationID string

// NewReservationID はサフィックスから予約IDを生成する。
func NewReservationID(suffix string) ReservationID {
	return ReservationID("rsv_" + suffix)
}

// ReservationRef は実データ（共有支出・追加収入）がどの予約から登録されたかを表す値オブジェクト。
//
// ゼロ値は「予約に由来しない（直接登録した）実データ」を表す。内部の予約IDは非公開で、
// 参照の有無は ID() の ok で判定する（空文字の予約IDという特別値を呼び出し側に意識させない）。
type ReservationRef struct {
	id ReservationID
}

// NewReservationRef は予約IDへの参照を生成する。id が空なら予約に由来しない参照（ゼロ値）を返す。
// 永続化した値（空文字を含む）から復元する用途を想定している。
func NewReservationRef(id ReservationID) ReservationRef {
	return ReservationRef{id: id}
}

// ID は参照先の予約IDを返す。予約に由来しない場合は ok=false。
func (r ReservationRef) ID() (id ReservationID, ok bool) {
	return r.id, r.id != ""
}

// Is は参照先が予約 id かどうかを返す。
func (r ReservationRef) Is(id ReservationID) bool { return id != "" && r.id == id }

// IsFromReservation は予約から登録された実データかどうかを返す。
func (r ReservationRef) IsFromReservation() bool { return r.id != "" }

// String は永続化・API表現用の文字列を返す。予約に由来しない場合は空文字。
func (r ReservationRef) String() string { return string(r.id) }

// Reservation は「発生する予定はあるが金額が確定していない」収入・支出の予約を表すエンティティ。
//
// 予約そのものは精算に影響しない。金額が確定したら対象月の共有支出（支出の予約）または
// 追加収入（収入の予約）として入力し、その実データが精算に反映される。
// Month がゼロ値なら毎月、値ありならその精算月のみ（単発）の予約。
type Reservation struct {
	ID              ReservationID
	Kind            ReservationKind
	MemberID        MemberID // 支出なら支払う人、収入なら収入を得る人
	Description     string
	EstimatedAmount Money     // 見込み額（円）。0 は未定
	Month           YearMonth // ゼロ値なら毎月
	// StartMonth は毎月の予約が始まる精算月。これより前の月には存在しない（登録前の月に未入力として現れないように）。
	// ゼロ値なら制限なし。単発の予約では使わない（Month のみ有効）。
	StartMonth YearMonth
	// SkippedMonths は「今月はなし」にした精算月（昇順・重複なし）。毎月の予約だけが持つ。
	// 予約自身に持たせることで、スキップ・解除・頻度変更・削除が予約1件の書き込みで完結する。
	SkippedMonths []YearMonth
}

// NewReservation は予約を生成する。month がゼロ値なら毎月の予約として扱い、start をその開始月にする
// （単発の予約では start は無視する）。SkippedMonths は空で生成する（永続化からの復元時は RestoreSkippedMonths で設定する）。
func NewReservation(id string, kind ReservationKind, memberID MemberID, description string, estimated Money, month, start YearMonth) (Reservation, error) {
	if id == "" {
		return Reservation{}, fmt.Errorf("%w: 予約IDは必須です", ErrValidation)
	}
	if _, err := ParseReservationKind(string(kind)); err != nil {
		return Reservation{}, err
	}
	if memberID == "" {
		return Reservation{}, fmt.Errorf("%w: メンバーIDは必須です", ErrValidation)
	}
	if strings.TrimSpace(description) == "" {
		return Reservation{}, fmt.Errorf("%w: 予約の内容は必須です", ErrValidation)
	}
	if estimated < 0 {
		return Reservation{}, fmt.Errorf("%w: 見込み額は0以上の整数（円）で指定してください: %d", ErrValidation, estimated)
	}
	if !month.IsZero() {
		start = YearMonth{}
	}
	return Reservation{
		ID:              ReservationID(id),
		Kind:            kind,
		MemberID:        memberID,
		Description:     strings.TrimSpace(description),
		EstimatedAmount: estimated,
		Month:           month,
		StartMonth:      start,
	}, nil
}

// RestoreSkippedMonths は永続化した「今月はなし」の月を設定した予約を返す（復元用）。
// 単発の予約はスキップを持てないため無視する。
func (r Reservation) RestoreSkippedMonths(months []YearMonth) Reservation {
	r.SkippedMonths = nil
	if !r.IsRecurring() {
		return r
	}
	for _, m := range months {
		r = r.withSkippedMonth(m)
	}
	return r
}

// IsRecurring は毎月の予約かどうかを返す。
func (r Reservation) IsRecurring() bool { return r.Month.IsZero() }

// Revise はメンバー・内容・見込み額・頻度（month がゼロ値なら毎月）を変更した予約を返す。IDと種別は変わらない。
//
// 毎月の開始月は start で指定する。start がゼロ値なら、毎月のままなら現在の開始月を、単発から毎月へ
// 変えるなら元の対象月を開始月にする。単発へ変更すると「今月はなし」の記録は消える（単発の予約は
// スキップを持てない）。毎月のまま変更する場合は記録を維持する。
func (r Reservation) Revise(memberID MemberID, description string, estimated Money, month, start YearMonth) (Reservation, error) {
	if start.IsZero() {
		if r.IsRecurring() {
			start = r.StartMonth
		} else {
			start = r.Month
		}
	}
	next, err := NewReservation(string(r.ID), r.Kind, memberID, description, estimated, month, start)
	if err != nil {
		return Reservation{}, err
	}
	return next.RestoreSkippedMonths(r.SkippedMonths), nil
}

// EnsureSkippable は予約を「今月はなし」にできるかを確認する。
//
// 「今月はなし」は毎月の予約を特定の月だけ見送るためのもの。単発（その月のみ）の予約は
// 発生しないなら予約自体が不要なので、スキップではなく削除する。
func (r Reservation) EnsureSkippable() error {
	if !r.IsRecurring() {
		return fmt.Errorf("%w: 今月だけの予約は「今月はなし」にできません。不要なら予約を削除してください", ErrValidation)
	}
	return nil
}

// IsSkippedIn は精算月 month が「今月はなし」になっているかを返す。
func (r Reservation) IsSkippedIn(month YearMonth) bool {
	for _, m := range r.SkippedMonths {
		if m == month {
			return true
		}
	}
	return false
}

// Skip は精算月 month を「今月はなし」にした予約を返す。毎月の予約で、month に存在する場合のみ可能。
func (r Reservation) Skip(month YearMonth) (Reservation, error) {
	if err := r.EnsureSkippable(); err != nil {
		return Reservation{}, err
	}
	if err := r.ensureAppliesTo(month); err != nil {
		return Reservation{}, err
	}
	return r.withSkippedMonth(month), nil
}

// Unskip は精算月 month の「今月はなし」を解除した予約を返す（記録がなければそのまま）。
func (r Reservation) Unskip(month YearMonth) Reservation {
	kept := make([]YearMonth, 0, len(r.SkippedMonths))
	for _, m := range r.SkippedMonths {
		if m != month {
			kept = append(kept, m)
		}
	}
	r.SkippedMonths = kept
	return r
}

// withSkippedMonth は month を昇順・重複なしで SkippedMonths に加える（元のスライスは変更しない）。
func (r Reservation) withSkippedMonth(month YearMonth) Reservation {
	if r.IsSkippedIn(month) {
		return r
	}
	months := append(append([]YearMonth{}, r.SkippedMonths...), month)
	sort.Slice(months, func(i, j int) bool { return months[i].String() < months[j].String() })
	r.SkippedMonths = months
	return r
}

// AppliesTo は予約が精算月 month に存在するか（開始月以降の毎月、または対象月が一致する単発か）を返す。
func (r Reservation) AppliesTo(month YearMonth) bool {
	if !r.IsRecurring() {
		return r.Month == month
	}
	return r.StartMonth.IsZero() || !month.Before(r.StartMonth)
}

// FulfillAsExpense は支出の予約に確定した金額を入力し、予約に紐づく共有支出を生成する。
//
// 支出日 date は締め日 cd に基づく精算月が month と一致しなければならない（別の月に計上されると
// その月の予約が入力済みにならないため）。支払者・内容は予約のものを引き継ぐ。
func (r Reservation) FulfillAsExpense(suffix string, month YearMonth, cd ClosingDay, amount Money, date time.Time, now time.Time) (Expense, error) {
	if r.Kind != ReservationKindExpense {
		return Expense{}, fmt.Errorf("%w: 収入の予約を支出として入力することはできません", ErrValidation)
	}
	if err := r.ensureAppliesTo(month); err != nil {
		return Expense{}, err
	}
	if cd.SettlementMonth(date) != month {
		return Expense{}, fmt.Errorf("%w: 支出日 %s は %s の精算期間に含まれません", ErrValidation, date.Format("2006-01-02"), month)
	}
	e, err := NewExpense(suffix, r.MemberID, amount, r.Description, date, now)
	if err != nil {
		return Expense{}, err
	}
	e.Reservation = NewReservationRef(r.ID)
	return e, nil
}

// FulfillAsIncome は収入の予約に確定した金額を入力し、予約に紐づく month のみの追加収入（単発）を生成する。
// 収入を得る人・内容は予約のものを引き継ぐ。
func (r Reservation) FulfillAsIncome(suffix string, month YearMonth, amount Money) (Income, error) {
	if r.Kind != ReservationKindIncome {
		return Income{}, fmt.Errorf("%w: 支出の予約を収入として入力することはできません", ErrValidation)
	}
	if err := r.ensureAppliesTo(month); err != nil {
		return Income{}, err
	}
	inc, err := NewIncome(string(NewOneOffIncomeID(month, suffix)), r.MemberID, amount, r.Description, month)
	if err != nil {
		return Income{}, err
	}
	inc.Reservation = NewReservationRef(r.ID)
	return inc, nil
}

func (r Reservation) ensureAppliesTo(month YearMonth) error {
	if !r.AppliesTo(month) {
		return fmt.Errorf("%w: この予約は %s には存在しません", ErrValidation, month)
	}
	return nil
}

// ReservationStatus は精算月ごとの予約の入力状況。
type ReservationStatus string

const (
	// ReservationPending は未入力（金額がまだ入力されていない）。精算確定時の警告対象。
	ReservationPending ReservationStatus = "pending"
	// ReservationFulfilled は入力済み（予約に紐づく支出・収入が対象月に存在する）。
	ReservationFulfilled ReservationStatus = "fulfilled"
	// ReservationSkipped は対象月は発生しないとしてスキップされた。
	ReservationSkipped ReservationStatus = "skipped"
)

// ReservationState は精算月における予約1件の入力状況。
type ReservationState struct {
	Reservation Reservation
	Status      ReservationStatus
	// FulfilledIDs は入力済みのとき、対象月に予約へ紐づく支出ID／収入ID（昇順）。
	// 通常は1件だが、締め日の変更などで同じ月に複数件が紐づく場合がある。
	FulfilledIDs []string
	// FulfilledAmount は入力済みのとき、対象月に紐づく支出／収入の合計額。
	FulfilledAmount Money
}

// WithFulfillment は金額を入力した（予約に紐づく実データ id を登録した）後の入力状況を返す。
// 入力済みは今月はなしより優先するため、スキップ済みだった場合も入力済みになる。
func (s ReservationState) WithFulfillment(id string, amount Money) ReservationState {
	s.Status = ReservationFulfilled
	s.FulfilledIDs = []string{id}
	s.FulfilledAmount = amount
	return s
}

// WithSkip は「今月はなし」を設定（skipped=true）または解除した後の入力状況を返す。
// 入力済みの状況はスキップ記録より優先するため変わらない。
func (s ReservationState) WithSkip(skipped bool) ReservationState {
	if s.Status == ReservationFulfilled {
		return s
	}
	if skipped {
		s.Status = ReservationSkipped
	} else {
		s.Status = ReservationPending
	}
	return s
}

// ResolveReservations は精算月 month に存在する予約それぞれの入力状況を判定する。
//
// expenses は精算月 month に計上される共有支出、incomes は month に適用される追加収入。
// 予約IDを持つ支出・収入が存在すれば入力済み、なければ予約が month を「今月はなし」にしているかを見て、
// どちらでもなければ未入力とする。入力済みはスキップより優先する。
// month に存在しない予約（別の月の単発）は結果に含めない。
func ResolveReservations(month YearMonth, reservations []Reservation, expenses []Expense, incomes []Income) []ReservationState {
	// 種別ごとに「予約ID → 紐づく実データ」を集計する。支出の予約は支出、収入の予約は収入とだけ照合する。
	expenseLinks := map[ReservationID]*reservationLink{}
	for _, e := range expenses {
		addReservationLink(expenseLinks, e.Reservation, string(e.ID), e.Amount)
	}
	incomeLinks := map[ReservationID]*reservationLink{}
	for _, inc := range incomes {
		addReservationLink(incomeLinks, inc.Reservation, string(inc.ID), inc.Amount)
	}

	states := make([]ReservationState, 0, len(reservations))
	for _, r := range reservations {
		if !r.AppliesTo(month) {
			continue
		}
		links := expenseLinks
		if r.Kind == ReservationKindIncome {
			links = incomeLinks
		}
		st := ReservationState{Reservation: r, Status: ReservationPending}
		if l, ok := links[r.ID]; ok {
			sort.Strings(l.ids)
			st.Status = ReservationFulfilled
			st.FulfilledIDs = l.ids
			st.FulfilledAmount = l.amount
		} else if r.IsSkippedIn(month) {
			st.Status = ReservationSkipped
		}
		states = append(states, st)
	}
	return states
}

// reservationLink は予約に紐づく実データ（支出・収入）のIDと合計額。
type reservationLink struct {
	ids    []string
	amount Money
}

func addReservationLink(links map[ReservationID]*reservationLink, ref ReservationRef, id string, amount Money) {
	rid, ok := ref.ID()
	if !ok {
		return // 予約に由来しない実データ
	}
	l, ok := links[rid]
	if !ok {
		l = &reservationLink{}
		links[rid] = l
	}
	l.ids = append(l.ids, id)
	l.amount += amount
}

// PendingReservations は入力状況のうち未入力のものだけを返す。
func PendingReservations(states []ReservationState) []ReservationState {
	var out []ReservationState
	for _, s := range states {
		if s.Status == ReservationPending {
			out = append(out, s)
		}
	}
	return out
}
