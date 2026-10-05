package web

// アカウント（認証中ユーザー自身の資格情報）API。

import (
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

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
