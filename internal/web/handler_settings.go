package web

// 精算設定（比重・締め日）API。

import (
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

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

// GetWeight godoc
//
//	@Summary		精算比重の取得
//	@Description	未設定時は 1:1。
//	@Tags			settings
//	@Produce		json
//	@Success		200	{object}	weightsDTO
//	@Failure		401	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/settings/weight [get]
func (h *Handler) GetWeight(w http.ResponseWriter, r *http.Request) {
	weight, err := h.settings.GetWeight(r.Context())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toWeightsDTO(h.couple, weight))
}

// closingDayDTO は締め日設定。
type closingDayDTO struct {
	ClosingDay int `json:"closingDay" example:"15"`
}

// GetClosingDay godoc
//
//	@Summary		締め日の取得
//	@Description	精算期間の起算日。未設定時は 1（暦月どおり）。
//	@Tags			settings
//	@Produce		json
//	@Success		200	{object}	closingDayDTO
//	@Failure		401	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/settings/closing-day [get]
func (h *Handler) GetClosingDay(w http.ResponseWriter, r *http.Request) {
	cd, err := h.settings.GetClosingDay(r.Context())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, closingDayDTO{ClosingDay: cd.Int()})
}

// UpdateClosingDay godoc
//
//	@Summary		締め日の更新
//	@Description	1〜31 を指定する。例: 15 なら (前月)15日〜(当月)14日を当月分として計上する。1 は暦月どおり。29〜31 は存在しない月では末日に丸める。
//	@Tags			settings
//	@Accept			json
//	@Produce		json
//	@Param			body	body		closingDayDTO	true	"締め日"
//	@Success		200		{object}	closingDayDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/settings/closing-day [put]
func (h *Handler) UpdateClosingDay(w http.ResponseWriter, r *http.Request) {
	var req closingDayDTO
	if !decodeBody(w, r, &req) {
		return
	}
	cd, err := h.settings.UpdateClosingDay(r.Context(), req.ClosingDay)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, closingDayDTO{ClosingDay: cd.Int()})
}

// UpdateWeight godoc
//
//	@Summary		精算比重の更新
//	@Description	各メンバーの比重（正整数）を memberID→比重 のマップで指定する。
//	@Tags			settings
//	@Accept			json
//	@Produce		json
//	@Param			body	body		weightsDTO	true	"比重"
//	@Success		200		{object}	weightsDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/settings/weight [put]
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
