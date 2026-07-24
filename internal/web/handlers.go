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
	Code    string `json:"code" example:"VALIDATION_ERROR"`
	Message string `json:"message" example:"validation error: 金額は1円以上で入力してください"`
}

// errorResponse はエラー時に返す JSON ボディ。
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
	ID    string `json:"id" example:"acct_9f3c1a2b7d4e5f60"`
	Name  string `json:"name" example:"太郎"`
	Color string `json:"color,omitempty" example:"#FF8800"`
}

func toMemberDTO(m domain.Member) memberDTO {
	return memberDTO{ID: string(m.ID), Name: m.Name}
}

func toMemberViewDTO(v application.MemberView) memberDTO {
	return memberDTO{ID: string(v.ID), Name: v.Name, Color: v.Color}
}

type expenseDTO struct {
	ID          string `json:"id" example:"2026-07_a1b2c3d4e5f6a7b8a1b2c3d4e5f6a7b8"`
	PaidBy      string `json:"paidBy" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen   int64  `json:"amountYen" example:"20000"`
	Description string `json:"description" example:"家賃"`
	Date        string `json:"date" example:"2026-07-01"`
	Month       string `json:"month" example:"2026-07"`
	CreatedAt   string `json:"createdAt" example:"2026-07-01T09:00:00Z"`
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
	MemberID  string `json:"memberId" example:"acct_9f3c1a2b7d4e5f60"`
	AmountYen int64  `json:"amountYen" example:"100000"`
}

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

// healthResponse はヘルスチェックのレスポンス。
type healthResponse struct {
	Status string `json:"status" example:"ok"`
}

// Health godoc
//
//	@Summary		ヘルスチェック
//	@Description	認証・クライアントキー検証の対象外。
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	healthResponse
//	@Router			/health [get]
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}

type loginRequest struct {
	MemberID string `json:"memberId" example:"taro"` // ログインID（可変のユーザー名）
	Password string `json:"password" example:"taro-password"`
}

type loginResponse struct {
	Token     string    `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	Member    memberDTO `json:"member"`
	ExpiresAt string    `json:"expiresAt" example:"2026-08-23T03:29:55Z"`
}

// Login godoc
//
//	@Summary		ログイン（JWT発行）
//	@Description	ログインID・パスワードを検証してJWTを発行する。memberId にはログインID（可変）を渡す。
//	@Description	JWT の subject は不変の AccountID。IP単位のレート制限があり、超過時は 429 を返す。
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		loginRequest	true	"認証情報"
//	@Success		200		{object}	loginResponse
//	@Failure		401		{object}	errorResponse	"ログインID/パスワード不一致"
//	@Failure		429		{object}	errorResponse	"レート制限超過"
//	@Router			/login [post]
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

// accountDTO は認証中アカウントの情報。accountId は不変、loginId は可変。
type accountDTO struct {
	AccountID string `json:"accountId" example:"acct_9f3c1a2b7d4e5f60"`
	LoginID   string `json:"loginId" example:"taro"`
	Name      string `json:"name" example:"太郎"`
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

// GetAccount godoc
//
//	@Summary		認証中アカウントの情報
//	@Description	不変の AccountID・可変のログインID・表示名を返す。
//	@Tags			account
//	@Produce		json
//	@Success		200	{object}	accountDTO
//	@Failure		401	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/account [get]
func (h *Handler) GetAccount(w http.ResponseWriter, r *http.Request) {
	id, _ := MemberIDFromContext(r.Context())
	h.accountResponse(w, r, id)
}

type updateLoginIDRequest struct {
	LoginID string `json:"loginId" example:"taro2"`
}

// UpdateLoginID godoc
//
//	@Summary		ログインIDの変更
//	@Description	ログイン用の可変ユーザー名を変更する。AccountID は不変で変わらない。英数字と . _ - のみ・32文字以内・2アカウントで重複不可。
//	@Tags			account
//	@Accept			json
//	@Produce		json
//	@Param			body	body		updateLoginIDRequest	true	"新しいログインID"
//	@Success		200		{object}	accountDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/account/login-id [put]
func (h *Handler) UpdateLoginID(w http.ResponseWriter, r *http.Request) {
	var req updateLoginIDRequest
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

type updatePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" example:"taro-password"`
	NewPassword     string `json:"newPassword" example:"new-password-8+"`
}

// UpdatePassword godoc
//
//	@Summary		パスワードの変更
//	@Description	現在のパスワードを検証したうえで新しいパスワード（8文字以上）に更新する。
//	@Tags			account
//	@Accept			json
//	@Param			body	body	updatePasswordRequest	true	"現在と新しいパスワード"
//	@Success		204		"変更成功"
//	@Failure		400		{object}	errorResponse	"現在のパスワード不一致・新パスワードが要件未満"
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/account/password [put]
func (h *Handler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	var req updatePasswordRequest
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

// membersResponse はメンバー一覧のレスポンス。
type membersResponse struct {
	Members []memberDTO `json:"members"`
}

// ListMembers godoc
//
//	@Summary		メンバー一覧（2人）
//	@Description	表示名・カラーの上書きを反映して2人のメンバーを返す。
//	@Tags			members
//	@Produce		json
//	@Success		200	{object}	membersResponse
//	@Failure		401	{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/members [get]
func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.settings.GetMembers(r.Context())
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, membersResponse{
		Members: []memberDTO{toMemberViewDTO(members[0]), toMemberViewDTO(members[1])},
	})
}

type updateMemberRequest struct {
	Name  *string `json:"name" example:"太郎"`
	Color *string `json:"color" example:"#FF8800"`
}

// UpdateMember godoc
//
//	@Summary		メンバーの表示名・カラーの更新
//	@Description	指定された項目のみ上書きする（省略した項目は変更しない）。
//	@Tags			members
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string				true	"メンバーID（AccountID）"
//	@Param			body	body		updateMemberRequest	true	"更新内容"
//	@Success		200		{object}	memberDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/members/{id} [put]
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
//	@Tags			expenses
//	@Accept			json
//	@Produce		json
//	@Param			id		path		string					true	"支出ID"
//	@Param			body	body		registerExpenseRequest	true	"支出"
//	@Success		200		{object}	expenseDTO
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
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

type inputIncomeRequest struct {
	AmountYen int64 `json:"amountYen" example:"100000"`
}

// incomeResponse は収入入力のレスポンス。
type incomeResponse struct {
	Month  string    `json:"month" example:"2026-07"`
	Income incomeDTO `json:"income"`
}

// InputIncome godoc
//
//	@Summary		月次収入の入力（上書き）
//	@Tags			incomes
//	@Accept			json
//	@Produce		json
//	@Param			month		path		string				true	"対象月（YYYY-MM）"
//	@Param			memberId	path		string				true	"メンバーID（AccountID）"
//	@Param			body		body		inputIncomeRequest	true	"収入額"
//	@Success		200			{object}	incomeResponse
//	@Failure		400			{object}	errorResponse
//	@Failure		401			{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/months/{month}/incomes/{memberId} [put]
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
	writeJSON(w, http.StatusOK, incomeResponse{
		Month:  income.Month.String(),
		Income: incomeDTO{MemberID: string(income.MemberID), AmountYen: int64(income.Amount)},
	})
}

// incomesResponse は対象月の収入一覧のレスポンス。
type incomesResponse struct {
	Month   string      `json:"month" example:"2026-07"`
	Incomes []incomeDTO `json:"incomes"`
}

// ListIncomes godoc
//
//	@Summary		月次収入の一覧
//	@Tags			incomes
//	@Produce		json
//	@Param			month	path		string	true	"対象月（YYYY-MM）"
//	@Success		200		{object}	incomesResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		401		{object}	errorResponse
//	@Security		BearerAuth
//	@Router			/months/{month}/incomes [get]
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
	writeJSON(w, http.StatusOK, incomesResponse{Month: r.PathValue("month"), Incomes: dtos})
}

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
	Transfer        *transferDTO          `json:"transfer"`
	Settled         bool                  `json:"settled" example:"false"`
}

// GetSettlement godoc
//
//	@Summary		月次精算の取得
//	@Description	比重に応じて双方の可処分所得が揃うよう振込額を算出する。収入が両者分そろっていない場合は 409（INCOME_NOT_READY）。
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

type settlementHistoryEntryDTO struct {
	Month           string       `json:"month" example:"2026-07"`
	Settled         bool         `json:"settled" example:"true"`
	TotalExpenseYen int64        `json:"totalExpenseYen" example:"40000"`
	Transfer        *transferDTO `json:"transfer"`
}

// settlementHistoryResponse は精算履歴のレスポンス。
type settlementHistoryResponse struct {
	Entries []settlementHistoryEntryDTO `json:"entries"`
}

// GetSettlementHistory godoc
//
//	@Summary		精算履歴の取得
//	@Description	from〜to（YYYY-MM）の精算履歴を新しい月順に返す。収入が揃っていない月は除外する。
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
	writeJSON(w, http.StatusOK, settlementHistoryResponse{Entries: dtos})
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
