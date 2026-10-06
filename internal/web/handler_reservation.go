package web

import (
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// reservationDTO は予約1件と、指定精算月における入力状況。
type reservationDTO struct {
	// ID は頻度に依存しない固定の予約ID（rsv_<hex>）。
	ID string `json:"id" example:"rsv_a1b2c3d4e5f6a7b8a1b2c3d4e5f6a7b8"`
	// Kind は "expense"（支出の予約）か "income"（収入の予約）。
	Kind string `json:"kind" example:"expense" enums:"expense,income"`
	// MemberID は支出なら支払う人、収入なら収入を得る人。
	MemberID    string `json:"memberId" example:"acct_9f3c1a2b7d4e5f60"`
	Description string `json:"description" example:"電気代"`
	Recurring   bool   `json:"recurring" example:"true"`
	Month       string `json:"month" example:""` // 毎月は空文字
	// StartMonth は毎月の予約の開始月。これより前の月には現れない。空文字は制限なし（単発は常に空文字）。
	StartMonth string `json:"startMonth" example:"2026-10"`
	// Status は指定精算月の入力状況。pending=未入力 / fulfilled=入力済み / skipped=今月はなし。
	Status string `json:"status" example:"pending" enums:"pending,fulfilled,skipped"`
	// FulfilledIDs は入力済みのとき、対象月に予約へ紐づく支出ID（kind=expense）／収入ID（kind=income）のすべて
	// （通常は1件）。入力を取り消すにはすべて削除する。
	FulfilledIDs []string `json:"fulfilledIds"`
	// FulfilledAmountYen は入力済みのとき、対象月に紐づく支出／収入の合計額。
	FulfilledAmountYen int64 `json:"fulfilledAmountYen" example:"0"`
}

func toReservationDTO(s domain.ReservationState) reservationDTO {
	r := s.Reservation
	month, start := "", ""
	if !r.IsRecurring() {
		month = r.Month.String()
	} else if !r.StartMonth.IsZero() {
		start = r.StartMonth.String()
	}
	return reservationDTO{
		ID:                 string(r.ID),
		Kind:               string(r.Kind),
		MemberID:           string(r.MemberID),
		Description:        r.Description,
		Recurring:          r.IsRecurring(),
		Month:              month,
		StartMonth:         start,
		Status:             string(s.Status),
		FulfilledIDs:       nonNil(s.FulfilledIDs),
		FulfilledAmountYen: int64(s.FulfilledAmount),
	}
}

type registerReservationRequest struct {
	// Kind は "expense"（支出の予約）か "income"（収入の予約）。更新時は無視される。
	Kind        string `json:"kind" example:"expense" enums:"expense,income"`
	MemberID    string `json:"memberId" example:"acct_9f3c1a2b7d4e5f60"`
	Description string `json:"description" example:"電気代"`
	// Month は空文字なら毎月、"YYYY-MM" ならその精算月のみの単発。更新時も指定した頻度へ変更する。
	Month string `json:"month" example:""`
	// StartMonth は毎月の予約の開始月（"YYYY-MM"）。これより前の月には現れない。登録時に空なら制限なし。
	// 更新時に空なら、毎月のままは現在の開始月、単発から毎月へ変えるなら元の対象月を開始月にする。
	StartMonth string `json:"startMonth" example:"2026-10"`
}

func (req registerReservationRequest) toInput() application.RegisterReservationInput {
	return application.RegisterReservationInput{
		Kind:        req.Kind,
		MemberID:    domain.MemberID(req.MemberID),
		Description: req.Description,
		Month:       req.Month,
		StartMonth:  req.StartMonth,
	}
}

// RegisterReservation godoc
//
//	@Summary		予約の登録
//	@Description	発生する予定はあるが金額が確定していない支出・収入を予約として登録する。予約自体は精算に影響しない。
//	@Description	month が空なら毎月（startMonth 以降の各月に現れる）、指定するとその精算月のみの単発。登録直後の入力状況は未入力（pending）。
//	@Tags			reservations
//	@Accept			json
//	@Produce		json
//	@Param			body	body		registerReservationRequest	true	"予約"
//	@Success		201		{object}	reservationDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/reservations [post]
func (h *Handler) RegisterReservation(w http.ResponseWriter, r *http.Request) {
	var req registerReservationRequest
	if !decodeBody(w, r, &req) {
		return
	}
	rsv, err := h.reserve.Register(r.Context(), req.toInput())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toReservationDTO(domain.ReservationState{Reservation: rsv, Status: domain.ReservationPending}))
}

// UpdateReservation godoc
//
//	@Summary		予約の更新
//	@Description	メンバー・内容・頻度（month が空なら毎月、指定するとその精算月のみ）を更新する。種別は変更できない。
//	@Description	頻度を変えても予約IDは変わらず、予約から登録済みの支出・収入との紐づけもそのまま有効。単発へ変更すると「今月はなし」の記録は消える。
//	@Description	レスポンスは予約の内容のみを表し、status 等の入力状況は反映しない（月ごとの入力状況は一覧 API で取得する）。
//	@Tags			reservations
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"予約ID"
//	@Param			body	body		registerReservationRequest	true	"予約"
//	@Success		200		{object}	reservationDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/reservations/{id} [put]
func (h *Handler) UpdateReservation(w http.ResponseWriter, r *http.Request) {
	var req registerReservationRequest
	if !decodeBody(w, r, &req) {
		return
	}
	rsv, err := h.reserve.Update(r.Context(), domain.ReservationID(r.PathValue("id")), req.toInput())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReservationDTO(domain.ReservationState{Reservation: rsv, Status: domain.ReservationPending}))
}

// reservationsResponse は指定月に存在する予約一覧のレスポンス。
type reservationsResponse struct {
	Month        string           `json:"month" example:"2026-10"`
	Reservations []reservationDTO `json:"reservations"`
	// PendingCount は未入力（status=pending）の予約の件数。精算確定前の警告に使う。
	PendingCount int `json:"pendingCount" example:"1"`
	// Settled は対象月が精算確定済みか。確定済みの月は予約の金額入力・今月はなしができない（409）。
	Settled bool `json:"settled" example:"false"`
}

// ListReservations godoc
//
//	@Summary		予約の一覧（入力状況つき）
//	@Description	指定精算月に存在する予約（毎月分＋当月単発分）と、その月の入力状況を返す。kind を指定すると種別で絞り込む。
//	@Description	精算を確定する前に pendingCount を確認し、未入力の予約があれば警告する用途を想定している。
//	@Tags			reservations
//	@Produce		json
//	@Param			month	query		string	true	"対象月（YYYY-MM）"
//	@Param			kind	query		string	false	"種別で絞り込む"	Enums(expense, income)
//	@Success		200		{object}	reservationsResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/reservations [get]
func (h *Handler) ListReservations(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	states, err := h.reserve.ListForMonth(r.Context(), month, r.URL.Query().Get("kind"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]reservationDTO, 0, len(states))
	for _, s := range states {
		dtos = append(dtos, toReservationDTO(s))
	}
	settled, err := h.settlement.IsSettled(r.Context(), month)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reservationsResponse{
		Month:        month,
		Reservations: dtos,
		PendingCount: len(domain.PendingReservations(states)),
		Settled:      settled,
	})
}

// DeleteReservation godoc
//
//	@Summary		予約の削除
//	@Description	予約を削除する。予約から登録済みの支出・収入は削除されずに残る。
//	@Tags			reservations
//	@Param			id	path	string	true	"予約ID"
//	@Success		204	"削除成功"
//	@Failure		401	{object}	errorResponse
//	@Failure		404	{object}	errorResponse
//	@Failure		409	{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/reservations/{id} [delete]
func (h *Handler) DeleteReservation(w http.ResponseWriter, r *http.Request) {
	if err := h.reserve.Delete(r.Context(), domain.ReservationID(r.PathValue("id"))); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type fulfillReservationRequest struct {
	Month     string `json:"month" example:"2026-10"`
	AmountYen int64  `json:"amountYen" example:"7820"`
	// Date は支出の予約のみ必須（YYYY-MM-DD）。month の精算期間内の日付を指定する。収入の予約では無視される。
	Date string `json:"date" example:"2026-10-25"`
}

// FulfillReservation godoc
//
//	@Summary		予約の金額入力
//	@Description	確定した金額を入力し、予約を実データとして登録する。支出の予約は共有支出（date の日付）、収入の予約はその月のみの追加収入として登録され、精算に反映される。
//	@Description	登録された支出・収入は reservationId で予約と紐づき、通常の支出・収入と同様に編集・削除できる（削除すると予約は未入力に戻る）。
//	@Description	既に入力済みの月には入力できない（400）。同時入力・再送による二重登録も「予約×月」の入力ロックで防ぎ、後着側は 400 になる。スキップ済みだった場合はスキップを解除する。
//	@Tags			reservations
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string						true	"予約ID"
//	@Param			body	body		fulfillReservationRequest	true	"確定した金額"
//	@Success		201		{object}	reservationDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/reservations/{id}/fulfill [post]
func (h *Handler) FulfillReservation(w http.ResponseWriter, r *http.Request) {
	var req fulfillReservationRequest
	if !decodeBody(w, r, &req) {
		return
	}
	state, err := h.reserve.Fulfill(r.Context(), domain.ReservationID(r.PathValue("id")), application.FulfillReservationInput{
		Month:     req.Month,
		AmountYen: req.AmountYen,
		Date:      req.Date,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toReservationDTO(state))
}

type skipReservationRequest struct {
	Month   string `json:"month" example:"2026-10"`
	Skipped bool   `json:"skipped" example:"true"`
}

// UpdateReservationSkip godoc
//
//	@Summary		予約のスキップ（今月はなし）
//	@Description	毎月の予約が指定精算月には発生しないことを記録する（skipped=false で解除）。スキップ済みの予約は未入力として数えない。
//	@Description	今月だけ（単発）の予約と入力済みの予約はスキップできない（400）。単発の予約が不要になった場合は削除する。解除は単発でも行える。
//	@Tags			reservations
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"予約ID"
//	@Param			body	body		skipReservationRequest	true	"スキップ状態"
//	@Success		200		{object}	reservationDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"精算確定済みの月 (MONTH_SETTLED)"
//	@Security		BearerAuth
//	@Router			/reservations/{id}/skip [put]
func (h *Handler) UpdateReservationSkip(w http.ResponseWriter, r *http.Request) {
	var req skipReservationRequest
	if !decodeBody(w, r, &req) {
		return
	}
	state, err := h.reserve.SetSkipped(r.Context(), domain.ReservationID(r.PathValue("id")), req.Month, req.Skipped)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReservationDTO(state))
}

// nonNil は JSON で null ではなく空配列を返すために nil を空スライスへ変換する。
func nonNil(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
