package web

// ハンドラ共通のレスポンス・エラー変換・リクエスト読み取り。

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

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
	case errors.Is(err, domain.ErrSettled):
		writeError(w, http.StatusConflict, "MONTH_SETTLED", err.Error())
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
