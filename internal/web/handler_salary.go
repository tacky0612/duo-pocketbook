package web

// 月次給与 API。

import (
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// salaryDTO はメンバーの月次給与（基本の収入）。
type salaryDTO struct {
	MemberID  string `json:"memberId" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen int64  `json:"amountYen" example:"100000"`
}

type inputSalaryRequest struct {
	AmountYen int64 `json:"amountYen" example:"100000"`
}

// salaryResponse は給与入力のレスポンス。
type salaryResponse struct {
	Month  string    `json:"month" example:"2026-07"`
	Salary salaryDTO `json:"salary"`
}

// InputSalary godoc
//
//	@Summary		月次給与の入力（上書き）
//	@Description	精算の可否判定に使う基本の収入。メンバーごと・月ごとに1件。
//	@Tags			salaries
//	@Accept			json
//	@Produce		json
//	@Param			month		path		string				true	"対象月（YYYY-MM）"
//	@Param			memberId	path		string				true	"メンバーID（AccountID）"
//	@Param			body		body		inputSalaryRequest	true	"給与額"
//	@Success		200			{object}	salaryResponse
//	@Failure		400			{object}	errorResponse
//	@Failure		401			{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/months/{month}/salaries/{memberId} [put]
func (h *Handler) InputSalary(w http.ResponseWriter, r *http.Request) {
	var req inputSalaryRequest
	if !decodeBody(w, r, &req) {
		return
	}
	salary, err := h.settlement.InputSalary(
		r.Context(), r.PathValue("month"), domain.MemberID(r.PathValue("memberId")), req.AmountYen,
	)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, salaryResponse{
		Month:  salary.Month.String(),
		Salary: salaryDTO{MemberID: string(salary.MemberID), AmountYen: int64(salary.Amount)},
	})
}

// salariesResponse は対象月の給与一覧のレスポンス。
type salariesResponse struct {
	Month    string      `json:"month" example:"2026-07"`
	Salaries []salaryDTO `json:"salaries"`
}

// ListSalaries godoc
//
//	@Summary		月次給与の一覧
//	@Tags			salaries
//	@Produce		json
//	@Param			month	path		string	true	"対象月（YYYY-MM）"
//	@Success		200		{object}	salariesResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/months/{month}/salaries [get]
func (h *Handler) ListSalaries(w http.ResponseWriter, r *http.Request) {
	list, err := h.settlement.GetSalaries(r.Context(), r.PathValue("month"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]salaryDTO, 0, len(list))
	for _, salary := range list {
		dtos = append(dtos, salaryDTO{MemberID: string(salary.MemberID), AmountYen: int64(salary.Amount)})
	}
	writeJSON(w, http.StatusOK, salariesResponse{Month: r.PathValue("month"), Salaries: dtos})
}
