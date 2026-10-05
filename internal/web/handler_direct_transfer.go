package web

// 立替精算（共有支出とは別の A→B 送金）API。

import (
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// directTransferDTO は立替精算。month が空文字なら毎月継続、値ありなら当該精算月のみの単発。
type directTransferDTO struct {
	ID          string `json:"id" example:"2026-07_a1b2c3d4e5f6a7b8a1b2c3d4e5f6a7b8"`
	From        string `json:"from" example:"acct_9f3c1a2b7d4e5f60"`
	To          string `json:"to" example:"acct_1a2b3c4d5e6f7a8b"`
	AmountYen   int64  `json:"amountYen" example:"5000"`
	Description string `json:"description" example:"立替の返済"`
	Recurring   bool   `json:"recurring" example:"false"`
	Month       string `json:"month" example:"2026-07"`
}

func toDirectTransferDTO(dt domain.DirectTransfer) directTransferDTO {
	month := ""
	if !dt.IsRecurring() {
		month = dt.Month.String()
	}
	return directTransferDTO{
		ID:          string(dt.ID),
		From:        string(dt.From),
		To:          string(dt.To),
		AmountYen:   int64(dt.Amount),
		Description: dt.Description,
		Recurring:   dt.IsRecurring(),
		Month:       month,
	}
}

type registerDirectTransferRequest struct {
	From        string `json:"from" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"5000"`
	Description string `json:"description" example:"立替の返済"`
	// Month は空文字なら毎月継続、"YYYY-MM" ならその精算月のみの単発として登録する。
	Month string `json:"month" example:"2026-07"`
}

// RegisterDirectTransfer godoc
//
//	@Summary		立替精算の登録
//	@Description	共有支出とは別に、送金元から他方へ渡す金額を登録する。比重按分に含めず、振込額へそのまま加算される。month が空なら毎月継続、指定するとその精算月のみの単発。
//	@Tags			direct-transfers
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerDirectTransferRequest	true	"立替精算"
//	@Success		201		{object}	directTransferDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/direct-transfers [post]
func (h *Handler) RegisterDirectTransfer(w http.ResponseWriter, r *http.Request) {
	var req registerDirectTransferRequest
	if !decodeBody(w, r, &req) {
		return
	}
	dt, err := h.direct.Register(r.Context(), application.RegisterDirectTransferInput{
		From:        domain.MemberID(req.From),
		AmountYen:   req.AmountYen,
		Description: req.Description,
		Month:       req.Month,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toDirectTransferDTO(dt))
}

// UpdateDirectTransfer godoc
//
//	@Summary		立替精算の更新
//	@Description	送金元・金額・内容・頻度を更新する。month を空にすると毎月継続、"YYYY-MM" を指定するとその月のみの単発に切り替わる（頻度変更時はIDが変わる）。
//	@Tags			direct-transfers
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string							true	"立替精算ID"
//	@Param			body	body		registerDirectTransferRequest	true	"立替精算"
//	@Success		200		{object}	directTransferDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/direct-transfers/{id} [put]
func (h *Handler) UpdateDirectTransfer(w http.ResponseWriter, r *http.Request) {
	var req registerDirectTransferRequest
	if !decodeBody(w, r, &req) {
		return
	}
	dt, err := h.direct.Update(r.Context(), domain.DirectTransferID(r.PathValue("id")), application.RegisterDirectTransferInput{
		From:        domain.MemberID(req.From),
		AmountYen:   req.AmountYen,
		Description: req.Description,
		Month:       req.Month,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toDirectTransferDTO(dt))
}

// directTransfersResponse は指定月に適用される立替精算一覧のレスポンス。
type directTransfersResponse struct {
	Month           string              `json:"month" example:"2026-07"`
	DirectTransfers []directTransferDTO `json:"directTransfers"`
}

// ListDirectTransfers godoc
//
//	@Summary		立替精算の一覧
//	@Description	指定精算月に適用される立替精算（毎月継続分＋当月単発分）を返す。
//	@Tags			direct-transfers
//	@Produce		json
//	@Param			month	query		string	true	"対象月（YYYY-MM）"
//	@Success		200		{object}	directTransfersResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/direct-transfers [get]
func (h *Handler) ListDirectTransfers(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	list, err := h.direct.ListForMonth(r.Context(), month)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]directTransferDTO, 0, len(list))
	for _, dt := range list {
		dtos = append(dtos, toDirectTransferDTO(dt))
	}
	writeJSON(w, http.StatusOK, directTransfersResponse{Month: month, DirectTransfers: dtos})
}

// DeleteDirectTransfer godoc
//
//	@Summary		立替精算の削除
//	@Tags			direct-transfers
//	@Param			id	path	string	true	"立替精算ID"
//	@Success		204	"削除成功"
//	@Failure		401	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/direct-transfers/{id} [delete]
func (h *Handler) DeleteDirectTransfer(w http.ResponseWriter, r *http.Request) {
	id := domain.DirectTransferID(r.PathValue("id"))
	if err := h.direct.Delete(r.Context(), id); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
