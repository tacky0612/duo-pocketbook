package web

// 月次精算・精算の確定状態・精算履歴 API。

import (
	"net/http"
	"time"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

type settlementMemberDTO struct {
	ID             string `json:"id" example:"acct_9f3c1a2b7d4e5f60"`
	Name           string `json:"name" example:"太郎"`
	Weight         int64  `json:"weight" example:"1"`
	IncomeYen      int64  `json:"incomeYen" example:"100000"`
	PaidExpenseYen int64  `json:"paidExpenseYen" example:"20000"`
	DisposableYen  int64  `json:"disposableYen" example:"55000"`
}

type transferDTO struct {
	From      string `json:"from" example:"acct_9f3c1a2b7d4e5f60"`
	To        string `json:"to" example:"acct_1a2b3c4d5e6f7a8b"`
	AmountYen int64  `json:"amountYen" example:"25000"`
}

type settlementResponse struct {
	Month           string                `json:"month" example:"2026-07"`
	TotalExpenseYen int64                 `json:"totalExpenseYen" example:"40000"`
	Members         []settlementMemberDTO `json:"members"`
	// Transfer は実際の振込（精算分＋立替精算分の合算）。0円なら null。
	Transfer *transferDTO `json:"transfer"`
	// SettlementTransfer は比重按分による精算分のみの振込。0円なら null。
	SettlementTransfer *transferDTO `json:"settlementTransfer"`
	// DirectTransfer は立替精算の純額のみの振込。0円なら null。
	DirectTransfer *transferDTO `json:"directTransfer"`
	// TotalDirectTransferYen は当月に適用された立替精算の総額（方向を問わない絶対額の合計）。
	TotalDirectTransferYen int64 `json:"totalDirectTransferYen" example:"5000"`
	Settled                bool  `json:"settled" example:"false"`
}

// toTransferDTO は domain.Transfer を DTO へ変換する（nil はそのまま nil）。
func toTransferDTO(t *domain.Transfer) *transferDTO {
	if t == nil {
		return nil
	}
	return &transferDTO{From: string(t.From), To: string(t.To), AmountYen: int64(t.Amount)}
}

// GetSettlement godoc
//
//	@Summary		月次精算の取得
//	@Description	比重に応じて双方の可処分所得が揃うよう振込額を算出する。給与が両者分そろっていない場合は 409（INCOME_NOT_READY）。
//	@Tags			settlement
//	@Produce		json
//	@Param			month	path		string	true	"対象月（YYYY-MM）"
//	@Success		200		{object}	settlementResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		409		{object}	errorResponse	"収入未入力"
//	@Security		BearerAuth
//	@Router			/months/{month}/settlement [get]
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
		Month:                  s.Month.String(),
		TotalExpenseYen:        int64(s.TotalExpense),
		Transfer:               toTransferDTO(s.Transfer),
		SettlementTransfer:     toTransferDTO(s.SettlementTransfer),
		DirectTransfer:         toTransferDTO(s.DirectTransfer),
		TotalDirectTransferYen: int64(s.TotalDirectTransfer),
		Settled:                settled,
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
	writeJSON(w, http.StatusOK, resp)
}

// snapshotExpenseDTO は精算スナップショットに含まれる共有支出1件の明細。
type snapshotExpenseDTO struct {
	PaidBy      string `json:"paidBy" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"80000"`
	Description string `json:"description" example:"家賃"`
	Date        string `json:"date" example:"2026-07-03"`
	Recurring   bool   `json:"recurring" example:"false"`
}

// snapshotDirectTransferDTO は精算スナップショットに含まれる立替精算1件の明細。
type snapshotDirectTransferDTO struct {
	From        string `json:"from" example:"acct_9f3c1a2b7d4e5f60"`
	To          string `json:"to" example:"acct_1a2b3c4d5e6f7a8b"`
	AmountYen   int64  `json:"amountYen" example:"5000"`
	Description string `json:"description" example:"立替分"`
	Recurring   bool   `json:"recurring" example:"false"`
}

// settlementHistoryEntryDTO は精算完了時点の内容を凍結したスナップショット1件。
type settlementHistoryEntryDTO struct {
	Month           string                `json:"month" example:"2026-07"`
	SettledAt       string                `json:"settledAt" example:"2026-07-31T12:00:00Z"`
	TotalExpenseYen int64                 `json:"totalExpenseYen" example:"40000"`
	Members         []settlementMemberDTO `json:"members"`
	// Transfer は実際の振込（精算分＋立替精算分の合算）。0円なら null。
	Transfer *transferDTO `json:"transfer"`
	// SettlementTransfer は比重按分による精算分のみの振込。0円なら null。
	SettlementTransfer *transferDTO `json:"settlementTransfer"`
	// DirectTransfer は立替精算の純額のみの振込。0円なら null。
	DirectTransfer *transferDTO `json:"directTransfer"`
	// TotalDirectTransferYen は当月に適用された立替精算の総額（方向を問わない絶対額の合計）。
	TotalDirectTransferYen int64                       `json:"totalDirectTransferYen" example:"5000"`
	Expenses               []snapshotExpenseDTO        `json:"expenses"`
	DirectTransfers        []snapshotDirectTransferDTO `json:"directTransfers"`
}

// settlementHistoryResponse は精算履歴のレスポンス。
type settlementHistoryResponse struct {
	Entries []settlementHistoryEntryDTO `json:"entries"`
}

// GetSettlementHistory godoc
//
//	@Summary		精算履歴の取得
//	@Description	from〜to（YYYY-MM）の精算スナップショット（精算完了時点の内容）を新しい月順に返す。精算未完了の月は除外する。
//	@Tags			settlement
//	@Produce		json
//	@Param			from	query		string	true	"開始月（YYYY-MM）"
//	@Param			to		query		string	true	"終了月（YYYY-MM）"
//	@Success		200		{object}	settlementHistoryResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/settlements/history [get]
func (h *Handler) GetSettlementHistory(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	snapshots, err := h.settlement.History(r.Context(), from, to)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	dtos := make([]settlementHistoryEntryDTO, 0, len(snapshots))
	for _, s := range snapshots {
		dtos = append(dtos, toSettlementHistoryEntryDTO(s))
	}
	writeJSON(w, http.StatusOK, settlementHistoryResponse{Entries: dtos})
}

// toSettlementHistoryEntryDTO は精算スナップショットを履歴エントリDTOへ変換する。
func toSettlementHistoryEntryDTO(s domain.SettlementSnapshot) settlementHistoryEntryDTO {
	set := s.Settlement
	dto := settlementHistoryEntryDTO{
		Month:                  set.Month.String(),
		SettledAt:              s.SettledAt.UTC().Format(time.RFC3339),
		TotalExpenseYen:        int64(set.TotalExpense),
		Transfer:               toTransferDTO(set.Transfer),
		SettlementTransfer:     toTransferDTO(set.SettlementTransfer),
		DirectTransfer:         toTransferDTO(set.DirectTransfer),
		TotalDirectTransferYen: int64(set.TotalDirectTransfer),
	}
	for _, m := range set.Members {
		dto.Members = append(dto.Members, settlementMemberDTO{
			ID:             string(m.Member.ID),
			Name:           m.Member.Name,
			Weight:         m.Weight,
			IncomeYen:      int64(m.Income),
			PaidExpenseYen: int64(m.PaidExpense),
			DisposableYen:  int64(m.Disposable),
		})
	}
	dto.Expenses = make([]snapshotExpenseDTO, 0, len(s.Expenses))
	for _, e := range s.Expenses {
		dto.Expenses = append(dto.Expenses, snapshotExpenseDTO{
			PaidBy:      string(e.PaidBy),
			AmountYen:   int64(e.Amount),
			Description: e.Description,
			Date:        e.Date,
			Recurring:   e.Recurring,
		})
	}
	dto.DirectTransfers = make([]snapshotDirectTransferDTO, 0, len(s.DirectTransfers))
	for _, d := range s.DirectTransfers {
		dto.DirectTransfers = append(dto.DirectTransfers, snapshotDirectTransferDTO{
			From:        string(d.From),
			To:          string(d.To),
			AmountYen:   int64(d.Amount),
			Description: d.Description,
			Recurring:   d.Recurring,
		})
	}
	return dto
}

type settlementStatusRequest struct {
	Settled bool `json:"settled" example:"true"`
}

// settlementStatusResponse は精算済みフラグ更新のレスポンス。
type settlementStatusResponse struct {
	Month   string `json:"month" example:"2026-07"`
	Settled bool   `json:"settled" example:"true"`
}

// UpdateSettlementStatus godoc
//
//	@Summary		精算済みフラグの更新
//	@Tags			settlement
//	@Accept			json
//	@Produce		json
//	@Param			month	path		string					true	"対象月（YYYY-MM）"
//	@Param			body	body		settlementStatusRequest	true	"精算済みフラグ"
//	@Success		200		{object}	settlementStatusResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/months/{month}/settlement/status [put]
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
	writeJSON(w, http.StatusOK, settlementStatusResponse{Month: month, Settled: settled})
}
