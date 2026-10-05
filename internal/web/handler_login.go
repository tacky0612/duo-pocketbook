package web

// ログイン（JWT 発行）API。

import (
	"net/http"
	"time"
)

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
