package web

// 固定費（毎月の共有支出）API。

import (
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

type recurringExpenseDTO struct {
	ID          string `json:"id" example:"recurring-a1b2c3d4"`
	PaidBy      string `json:"paidBy" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"80000"`
	Description string `json:"description" example:"家賃"`
}

func toRecurringExpenseDTO(e domain.RecurringExpense) recurringExpenseDTO {
	return recurringExpenseDTO{
		ID:          string(e.ID),
		PaidBy:      string(e.PaidBy),
		AmountYen:   int64(e.Amount),
		Description: e.Description,
	}
}

type registerRecurringExpenseRequest struct {
	PaidBy      string `json:"paidBy" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"80000"`
	Description string `json:"description" example:"家賃"`
}

// RegisterRecurringExpense godoc
//
//	@Summary		固定費の登録
//	@Tags			recurring-expenses
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerRecurringExpenseRequest	true	"固定費"
//	@Success		201		{object}	recurringExpenseDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/recurring-expenses [post]
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

// UpdateRecurringExpense godoc
//
//	@Summary		固定費の更新
//	@Tags			recurring-expenses
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string							true	"固定費ID"
//	@Param			body	body		registerRecurringExpenseRequest	true	"固定費"
//	@Success		200		{object}	recurringExpenseDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/recurring-expenses/{id} [put]
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

// recurringExpensesResponse は固定費一覧のレスポンス。
type recurringExpensesResponse struct {
	RecurringExpenses []recurringExpenseDTO `json:"recurringExpenses"`
}

// ListRecurringExpenses godoc
//
//	@Summary		固定費の一覧
//	@Tags			recurring-expenses
//	@Produce		json
//	@Success		200	{object}	recurringExpensesResponse
//	@Failure		401	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/recurring-expenses [get]
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
	writeJSON(w, http.StatusOK, recurringExpensesResponse{RecurringExpenses: dtos})
}

// DeleteRecurringExpense godoc
//
//	@Summary		固定費の削除
//	@Tags			recurring-expenses
//	@Param			id	path	string	true	"固定費ID"
//	@Success		204	"削除成功"
//	@Failure		401	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/recurring-expenses/{id} [delete]
func (h *Handler) DeleteRecurringExpense(w http.ResponseWriter, r *http.Request) {
	id := domain.RecurringExpenseID(r.PathValue("id"))
	if err := h.recurring.Delete(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
