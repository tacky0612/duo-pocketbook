// Package web はAPIインターフェイス（HTTPハンドラ・ルーティング・認証）を定義する。
// リクエスト/レスポンスの変換のみを担い、業務ロジックはアプリケーション層に委譲する。
package web

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// Handler はAPIハンドラ群。
type Handler struct {
	couple     domain.Couple
	auth       *Authenticator
	account    *application.AccountUsecase
	expenses   *application.ExpenseUsecase
	settlement *application.SettlementUsecase
	settings   *application.SettingsUsecase
	recurring  *application.RecurringExpenseUsecase
}

// NewHandler は Handler を生成する。
func NewHandler(
	couple domain.Couple,
	auth *Authenticator,
	account *application.AccountUsecase,
	expenses *application.ExpenseUsecase,
	settlement *application.SettlementUsecase,
	settings *application.SettingsUsecase,
	recurring *application.RecurringExpenseUsecase,
) *Handler {
	return &Handler{
		couple:     couple,
		auth:       auth,
		account:    account,
		expenses:   expenses,
		settlement: settlement,
		settings:   settings,
		recurring:  recurring,
	}
}

// ---- 共通レスポンス ----

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body != nil {
		if err := json.NewEncoder(w).Encode(body); err != nil {
			slog.Error("レスポンスの書き込みに失敗", "error", err)
		}
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

// writeUsecaseError はアプリケーション層のエラーをHTTPステータスへ変換する。
func writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrIncomeNotReady):
		writeError(w, http.StatusConflict, "INCOME_NOT_READY", err.Error())
	case errors.Is(err, domain.ErrValidation):
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	case errors.Is(err, application.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", "対象のデータが見つかりません")
	default:
		slog.Error("内部エラー", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "内部エラーが発生しました")
	}
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "リクエストボディのJSONが不正です")
		return false
	}
	return true
}

// ---- DTO ----

type memberDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

func toMemberDTO(m domain.Member) memberDTO {
	return memberDTO{ID: string(m.ID), Name: m.Name}
}

func toMemberViewDTO(v application.MemberView) memberDTO {
	return memberDTO{ID: string(v.ID), Name: v.Name, Color: v.Color}
}

type expenseDTO struct {
	ID          string `json:"id"`
	PaidBy      string `json:"paidBy"`
	AmountYen   int64  `json:"amountYen"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Month       string `json:"month"`
	CreatedAt   string `json:"createdAt"`
}

func toExpenseDTO(e domain.Expense) expenseDTO {
	return expenseDTO{
		ID:          string(e.ID),
		PaidBy:      string(e.PaidBy),
		AmountYen:   int64(e.Amount),
		Description: e.Description,
		Date:        e.Date.Format("2006-01-02"),
		Month:       e.Month().String(),
		CreatedAt:   e.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type incomeDTO struct {
	MemberID  string `json:"memberId"`
	AmountYen int64  `json:"amountYen"`
}

type recurringExpenseDTO struct {
	ID          string `json:"id"`
	PaidBy      string `json:"paidBy"`
	AmountYen   int64  `json:"amountYen"`
	Description string `json:"description"`
}

func toRecurringExpenseDTO(e domain.RecurringExpense) recurringExpenseDTO {
	return recurringExpenseDTO{
		ID:          string(e.ID),
		PaidBy:      string(e.PaidBy),
		AmountYen:   int64(e.Amount),
		Description: e.Description,
	}
}

type weightsDTO struct {
	Weights map[string]int64 `json:"weights"`
}

func toWeightsDTO(couple domain.Couple, w domain.Weight) weightsDTO {
	dto := weightsDTO{Weights: map[string]int64{}}
	for _, m := range couple.Members() {
		if v, ok := w.Of(m.ID); ok {
			dto.Weights[string(m.ID)] = v
		}
	}
	return dto
}

// ---- ハンドラ ----

// Health はヘルスチェック。
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type loginRequest struct {
	MemberID string `json:"memberId"` // ログインID
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	Member    memberDTO `json:"member"`
	ExpiresAt string    `json:"expiresAt"`
}

// Login はログインID・パスワードを検証してJWTを発行する。
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeBody(w, r, &req) {
		return
	}
	accountID, err := h.account.Authenticate(r.Context(), req.MemberID, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "ログインIDまたはパスワードが違います")
		return
	}
	member, _ := h.couple.Get(accountID)
	token, expiresAt, err := h.auth.IssueToken(accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "内部エラーが発生しました")
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{
		Token:     token,
		Member:    toMemberDTO(member),
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

// ---- アカウント（自分の資格情報） ----

type accountDTO struct {
	AccountID string `json:"accountId"`
	LoginID   string `json:"loginId"`
	Name      string `json:"name"`
}

func (h *Handler) accountResponse(w http.ResponseWriter, r *http.Request, id domain.MemberID) {
	acc, err := h.account.Get(r.Context(), id)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	member, _ := h.couple.Get(id)
	writeJSON(w, http.StatusOK, accountDTO{AccountID: string(acc.ID), LoginID: acc.LoginID, Name: member.Name})
}

// GetAccount は認証中アカウントの AccountID・ログインID・表示名を返す。
func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := MemberIDFromContext(r.Context())
	h.accountResponse(w, r, id)
}

// UpdateLoginID はログインIDを変更する。
func (h *Handler) UpdateLoginID(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LoginID string `json:"loginId"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	id, _ := MemberIDFromContext(r.Context())
	if err := h.account.UpdateLoginID(r.Context(), id, req.LoginID); err != nil {
		writeUsecaseError(w, err)
		return
	}
	h.accountResponse(w, r, id)
}

// UpdatePassword はパスワードを変更する（現在のパスワードを検証）。
func (h *Handler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	id, _ := MemberIDFromContext(r.Context())
	if err := h.account.UpdatePassword(r.Context(), id, req.CurrentPassword, req.NewPassword); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers はメンバー一覧を返す（表示名・カラーの上書きを反映）。
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.settings.GetMembers(r.Context())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]memberDTO{
		"members": {toMemberViewDTO(members[0]), toMemberViewDTO(members[1])},
	})
}

type updateMemberRequest struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

// UpdateMember はメンバーの表示名・カラーを更新する（指定された項目のみ）。
func (h *Handler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	var req updateMemberRequest
	if !decodeBody(w, r, &req) {
		return
	}
	id := domain.MemberID(r.PathValue("id"))
	if req.Name != nil {
		if err := h.settings.UpdateMemberName(r.Context(), id, *req.Name); err != nil {
			writeUsecaseError(w, err)
			return
		}
	}
	if req.Color != nil {
		if err := h.settings.UpdateMemberColor(r.Context(), id, *req.Color); err != nil {
			writeUsecaseError(w, err)
			return
		}
	}
	member, err := h.settings.GetMember(r.Context(), id)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toMemberViewDTO(member))
}

type registerExpenseRequest struct {
	PaidBy      string `json:"paidBy"`
	AmountYen   int64  `json:"amountYen"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

// RegisterExpense は共有支出を登録する。
func (h *Handler) RegisterExpense(w http.ResponseWriter, r *http.Request) {
	var req registerExpenseRequest
	if !decodeBody(w, r, &req) {
		return
	}
	e, err := h.expenses.Register(r.Context(), application.RegisterExpenseInput{
		PaidBy:      domain.MemberID(req.PaidBy),
		AmountYen:   req.AmountYen,
		Description: req.Description,
		Date:        req.Date,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toExpenseDTO(e))
}

// UpdateExpense は既存の共有支出を更新する。
func (h *Handler) UpdateExpense(w http.ResponseWriter, r *http.Request) {
	var req registerExpenseRequest
	if !decodeBody(w, r, &req) {
		return
	}
	e, err := h.expenses.Update(r.Context(), domain.ExpenseID(r.PathValue("id")), application.RegisterExpenseInput{
		PaidBy:      domain.MemberID(req.PaidBy),
		AmountYen:   req.AmountYen,
		Description: req.Description,
		Date:        req.Date,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toExpenseDTO(e))
}

// ListExpenses は対象月の共有支出一覧を返す。
func (h *Handler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	list, err := h.expenses.ListByMonth(r.Context(), month)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]expenseDTO, 0, len(list))
	for _, e := range list {
		dtos = append(dtos, toExpenseDTO(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": month, "expenses": dtos})
}

// DeleteExpense は共有支出を削除する。
func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	id := domain.ExpenseID(r.PathValue("id"))
	if err := h.expenses.Delete(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type inputIncomeRequest struct {
	AmountYen int64 `json:"amountYen"`
}

// InputIncome は対象月のメンバーの収入を入力する。
func (h *Handler) InputIncome(w http.ResponseWriter, r *http.Request) {
	var req inputIncomeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	income, err := h.settlement.InputIncome(
		r.Context(), r.PathValue("month"), domain.MemberID(r.PathValue("memberId")), req.AmountYen,
	)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"month":  income.Month.String(),
		"income": incomeDTO{MemberID: string(income.MemberID), AmountYen: int64(income.Amount)},
	})
}

// ListIncomes は対象月の入力済み収入を返す。
func (h *Handler) ListIncomes(w http.ResponseWriter, r *http.Request) {
	list, err := h.settlement.GetIncomes(r.Context(), r.PathValue("month"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]incomeDTO, 0, len(list))
	for _, income := range list {
		dtos = append(dtos, incomeDTO{MemberID: string(income.MemberID), AmountYen: int64(income.Amount)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": r.PathValue("month"), "incomes": dtos})
}

type settlementMemberDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Weight         int64  `json:"weight"`
	IncomeYen      int64  `json:"incomeYen"`
	PaidExpenseYen int64  `json:"paidExpenseYen"`
	DisposableYen  int64  `json:"disposableYen"`
}

type transferDTO struct {
	From      string `json:"from"`
	To        string `json:"to"`
	AmountYen int64  `json:"amountYen"`
}

type settlementResponse struct {
	Month           string                `json:"month"`
	TotalExpenseYen int64                 `json:"totalExpenseYen"`
	Members         []settlementMemberDTO `json:"members"`
	Transfer        *transferDTO          `json:"transfer"`
	Settled         bool                  `json:"settled"`
}

// GetSettlement は対象月の精算結果を返す。
func (h *Handler) GetSettlement(w http.ResponseWriter, r *http.Request) {
	month := r.PathValue("month")
	s, err := h.settlement.GetSettlement(r.Context(), month)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	settled, err := h.settlement.IsSettled(r.Context(), month)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	resp := settlementResponse{
		Month:           s.Month.String(),
		TotalExpenseYen: int64(s.TotalExpense),
		Settled:         settled,
	}
	for _, m := range s.Members {
		resp.Members = append(resp.Members, settlementMemberDTO{
			ID:             string(m.Member.ID),
			Name:           m.Member.Name,
			Weight:         m.Weight,
			IncomeYen:      int64(m.Income),
			PaidExpenseYen: int64(m.PaidExpense),
			DisposableYen:  int64(m.Disposable),
		})
	}
	if s.Transfer != nil {
		resp.Transfer = &transferDTO{
			From:      string(s.Transfer.From),
			To:        string(s.Transfer.To),
			AmountYen: int64(s.Transfer.Amount),
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type registerRecurringExpenseRequest struct {
	PaidBy      string `json:"paidBy"`
	AmountYen   int64  `json:"amountYen"`
	Description string `json:"description"`
}

// RegisterRecurringExpense は固定費を登録する。
func (h *Handler) RegisterRecurringExpense(w http.ResponseWriter, r *http.Request) {
	var req registerRecurringExpenseRequest
	if !decodeBody(w, r, &req) {
		return
	}
	e, err := h.recurring.Register(r.Context(), application.RegisterRecurringExpenseInput{
		PaidBy:      domain.MemberID(req.PaidBy),
		AmountYen:   req.AmountYen,
		Description: req.Description,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRecurringExpenseDTO(e))
}

// UpdateRecurringExpense は既存の固定費を更新する。
func (h *Handler) UpdateRecurringExpense(w http.ResponseWriter, r *http.Request) {
	var req registerRecurringExpenseRequest
	if !decodeBody(w, r, &req) {
		return
	}
	e, err := h.recurring.Update(r.Context(), domain.RecurringExpenseID(r.PathValue("id")), application.RegisterRecurringExpenseInput{
		PaidBy:      domain.MemberID(req.PaidBy),
		AmountYen:   req.AmountYen,
		Description: req.Description,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toRecurringExpenseDTO(e))
}

// ListRecurringExpenses は固定費の一覧を返す。
func (h *Handler) ListRecurringExpenses(w http.ResponseWriter, r *http.Request) {
	list, err := h.recurring.List(r.Context())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]recurringExpenseDTO, 0, len(list))
	for _, e := range list {
		dtos = append(dtos, toRecurringExpenseDTO(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"recurringExpenses": dtos})
}

// DeleteRecurringExpense は固定費を削除する。
func (h *Handler) DeleteRecurringExpense(w http.ResponseWriter, r *http.Request) {
	id := domain.RecurringExpenseID(r.PathValue("id"))
	if err := h.recurring.Delete(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type settlementHistoryEntryDTO struct {
	Month           string       `json:"month"`
	Settled         bool         `json:"settled"`
	TotalExpenseYen int64        `json:"totalExpenseYen"`
	Transfer        *transferDTO `json:"transfer"`
}

// GetSettlementHistory は from〜to（YYYY-MM）の精算履歴を新しい月順に返す。
func (h *Handler) GetSettlementHistory(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	entries, err := h.settlement.History(r.Context(), from, to)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]settlementHistoryEntryDTO, 0, len(entries))
	for _, e := range entries {
		dto := settlementHistoryEntryDTO{
			Month:           e.Settlement.Month.String(),
			Settled:         e.Settled,
			TotalExpenseYen: int64(e.Settlement.TotalExpense),
		}
		if e.Settlement.Transfer != nil {
			dto.Transfer = &transferDTO{
				From:      string(e.Settlement.Transfer.From),
				To:        string(e.Settlement.Transfer.To),
				AmountYen: int64(e.Settlement.Transfer.Amount),
			}
		}
		dtos = append(dtos, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": dtos})
}

type settlementStatusRequest struct {
	Settled bool `json:"settled"`
}

// UpdateSettlementStatus は対象月の精算済みフラグを更新する。
func (h *Handler) UpdateSettlementStatus(w http.ResponseWriter, r *http.Request) {
	var req settlementStatusRequest
	if !decodeBody(w, r, &req) {
		return
	}
	month := r.PathValue("month")
	settled, err := h.settlement.SetSettled(r.Context(), month, req.Settled)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": month, "settled": settled})
}

// GetWeight は精算比重を返す。
func (h *Handler) GetWeight(w http.ResponseWriter, r *http.Request) {
	weight, err := h.settings.GetWeight(r.Context())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWeightsDTO(h.couple, weight))
}

// UpdateWeight は精算比重を更新する。
func (h *Handler) UpdateWeight(w http.ResponseWriter, r *http.Request) {
	var req weightsDTO
	if !decodeBody(w, r, &req) {
		return
	}
	in := application.UpdateWeightInput{Weights: map[domain.MemberID]int64{}}
	for id, v := range req.Weights {
		in.Weights[domain.MemberID(id)] = v
	}
	weight, err := h.settings.UpdateWeight(r.Context(), in)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWeightsDTO(h.couple, weight))
}
