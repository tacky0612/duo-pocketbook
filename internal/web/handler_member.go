package web

// メンバー（表示名・カラー）API。

import (
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

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
