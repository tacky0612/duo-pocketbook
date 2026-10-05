package web

// 共有支出 API。

import (
	"net/http"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

type expenseDTO struct {
	ID          string `json:"id" example:"2026-07_a1b2c3d4e5f6a7b8a1b2c3d4e5f6a7b8"`
	PaidBy      string `json:"paidBy" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"20000"`
	Description string `json:"description" example:"家賃"`
	Date        string `json:"date" example:"2026-07-01"`
	Month       string `json:"month" example:"2026-07"`
	CreatedAt   string `json:"createdAt" example:"2026-07-01T09:00:00Z"`
	// ReservationID は予約の入力で登録された支出ならその予約ID。通常の支出は空文字。
	ReservationID string `json:"reservationId" example:""`
}

func toExpenseDTO(e domain.Expense) expenseDTO {
	return expenseDTO{
		ID:            string(e.ID),
		PaidBy:        string(e.PaidBy),
		AmountYen:     int64(e.Amount),
		Description:   e.Description,
		Date:          e.Date.Format("2006-01-02"),
		Month:         e.Month().String(),
		CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
		ReservationID: e.Reservation.String(),
	}
}

type registerExpenseRequest struct {
	PaidBy      string `json:"paidBy" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"20000"`
	Description string `json:"description" example:"家賃"`
	Date        string `json:"date" example:"2026-07-01"`
}

// RegisterExpense godoc
//
//	@Summary		共有支出の登録
//	@Tags			expenses
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerExpenseRequest	true	"支出"
//	@Success		201		{object}	expenseDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/expenses [post]
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

// UpdateExpense godoc
//
//	@Summary		共有支出の更新
//	@Description	予約から登録した支出（reservationId あり）は、日付の変更で別の精算月へ移すことはできない（400）。紐づく予約が削除済みなら紐づけを外して通常の支出として更新する。
//	@Tags			expenses
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"支出ID"
//	@Param			body	body		registerExpenseRequest	true	"支出"
//	@Success		200		{object}	expenseDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/expenses/{id} [put]
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

// expensesResponse は対象月の共有支出一覧のレスポンス。
type expensesResponse struct {
	Month    string       `json:"month" example:"2026-07"`
	Expenses []expenseDTO `json:"expenses"`
}

// ListExpenses godoc
//
//	@Summary		共有支出の月別一覧
//	@Description	日付降順で返す。
//	@Tags			expenses
//	@Produce		json
//	@Param			month	query		string	true	"対象月（YYYY-MM）"
//	@Success		200		{object}	expensesResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/expenses [get]
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
	writeJSON(w, http.StatusOK, expensesResponse{Month: month, Expenses: dtos})
}

// DeleteExpense godoc
//
//	@Summary		共有支出の削除
//	@Tags			expenses
//	@Param			id	path	string	true	"支出ID"
//	@Success		204	"削除成功"
//	@Failure		401	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/expenses/{id} [delete]
func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	id := domain.ExpenseID(r.PathValue("id"))
	if err := h.expenses.Delete(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
