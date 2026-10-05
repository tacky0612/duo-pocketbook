package web

// システム系 API（ヘルスチェック）。

import (
	"net/http"
)

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
