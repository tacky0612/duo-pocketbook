package web

// 追加収入（給与とは別の収入）API。

import (
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// incomeDTO は給与とは別の追加収入。month が空文字なら毎月継続、値ありなら当該精算月のみの単発。
type incomeDTO struct {
	ID          string `json:"id" example:"2026-07_a1b2c3d4e5f6a7b8a1b2c3d4e5f6a7b8"`
	MemberID    string `json:"memberId" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"30000"`
	Description string `json:"description" example:"副業"`
	Recurring   bool   `json:"recurring" example:"false"`
	Month       string `json:"month" example:"2026-07"`
	// ReservationID は予約の入力で登録された収入ならその予約ID。通常の収入は空文字。
	ReservationID string `json:"reservationId" example:""`
}

func toIncomeDTO(inc domain.Income) incomeDTO {
	month := ""
	if !inc.IsRecurring() {
		month = inc.Month.String()
	}
	return incomeDTO{
		ID:            string(inc.ID),
		MemberID:      string(inc.MemberID),
		AmountYen:     int64(inc.Amount),
		Description:   inc.Description,
		Recurring:     inc.IsRecurring(),
		Month:         month,
		ReservationID: inc.Reservation.String(),
	}
}

type registerIncomeRequest struct {
	MemberID    string `json:"memberId" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"30000"`
	Description string `json:"description" example:"副業"`
	// Month は空文字なら毎月継続、"YYYY-MM" ならその精算月のみの単発として登録する。
	Month string `json:"month" example:"2026-07"`
}

// RegisterIncome godoc
//
//	@Summary		追加収入の登録
//	@Description	給与とは別の収入（副業など）を登録する。給与と合算して精算に反映される。month が空なら毎月継続、指定するとその精算月のみの単発。日付は持たない。
//	@Tags			incomes
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerIncomeRequest	true	"追加収入"
//	@Success		201		{object}	incomeDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/incomes [post]
func (h *Handler) RegisterIncome(w http.ResponseWriter, r *http.Request) {
	var req registerIncomeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	inc, err := h.income.Register(r.Context(), application.RegisterIncomeInput{
		MemberID:    domain.MemberID(req.MemberID),
		AmountYen:   req.AmountYen,
		Description: req.Description,
		Month:       req.Month,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toIncomeDTO(inc))
}

// UpdateIncome godoc
//
//	@Summary		追加収入の更新
//	@Description	メンバー・金額・内容を更新する。継続/単発の別と対象月は変更できない（変更するには削除して再登録する）。
//	@Tags			incomes
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"収入ID"
//	@Param			body	body		registerIncomeRequest	true	"追加収入"
//	@Success		200		{object}	incomeDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/incomes/{id} [put]
func (h *Handler) UpdateIncome(w http.ResponseWriter, r *http.Request) {
	var req registerIncomeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	inc, err := h.income.Update(r.Context(), domain.IncomeID(r.PathValue("id")), application.RegisterIncomeInput{
		MemberID:    domain.MemberID(req.MemberID),
		AmountYen:   req.AmountYen,
		Description: req.Description,
		Month:       req.Month,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toIncomeDTO(inc))
}

// incomesResponse は指定月に適用される追加収入一覧のレスポンス。
type incomesResponse struct {
	Month   string      `json:"month" example:"2026-07"`
	Incomes []incomeDTO `json:"incomes"`
}

// ListIncomes godoc
//
//	@Summary		追加収入の一覧
//	@Description	指定精算月に適用される追加収入（毎月継続分＋当月単発分）を返す。
//	@Tags			incomes
//	@Produce		json
//	@Param			month	query		string	true	"対象月（YYYY-MM）"
//	@Success		200		{object}	incomesResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/incomes [get]
func (h *Handler) ListIncomes(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	list, err := h.income.ListForMonth(r.Context(), month)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]incomeDTO, 0, len(list))
	for _, inc := range list {
		dtos = append(dtos, toIncomeDTO(inc))
	}
	writeJSON(w, http.StatusOK, incomesResponse{Month: month, Incomes: dtos})
}

// DeleteIncome godoc
//
//	@Summary		追加収入の削除
//	@Tags			incomes
//	@Param			id	path	string	true	"収入ID"
//	@Success		204	"削除成功"
//	@Failure		401	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/incomes/{id} [delete]
func (h *Handler) DeleteIncome(w http.ResponseWriter, r *http.Request) {
	id := domain.IncomeID(r.PathValue("id"))
	if err := h.income.Delete(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
