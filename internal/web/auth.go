package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ErrUnauthorized は認証失敗を表すエラー。
var ErrUnauthorized = errors.New("認証に失敗しました")

type contextKey string

const memberIDKey contextKey = "memberID"

// MemberIDFromContext はリクエストコンテキストから認証済みアカウントID（AccountID）を取り出す。
func MemberIDFromContext(ctx context.Context) (domain.MemberID, bool) {
	id, ok := ctx.Value(memberIDKey).(domain.MemberID)
	return id, ok
}

// Authenticator は JWT の発行・検証を担う。
// 認証（ログインID・パスワード照合）は application.AccountUsecase が担い、
// ここでは検証済みの AccountID をトークン化する。
type Authenticator struct {
	couple domain.Couple
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewAuthenticator は Authenticator を生成する。
func NewAuthenticator(secret string, ttl time.Duration, couple domain.Couple, now func() time.Time) *Authenticator {
	if now == nil {
		now = time.Now
	}
	return &Authenticator{
		couple: couple,
		secret: []byte(secret),
		ttl:    ttl,
		now:    now,
	}
}

// IssueToken は AccountID を subject とする JWT を発行する。
func (a *Authenticator) IssueToken(accountID domain.MemberID) (token string, expiresAt time.Time, err error) {
	expiresAt = a.now().Add(a.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   string(accountID),
		IssuedAt:  jwt.NewNumericDate(a.now()),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("トークンの発行に失敗しました: %w", err)
	}
	return token, expiresAt, nil
}

// Verify はJWTを検証して AccountID を返す。
func (a *Authenticator) Verify(token string) (domain.MemberID, error) {
	parsed, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	}, jwt.WithTimeFunc(a.now))
	if err != nil || !parsed.Valid {
		return "", ErrUnauthorized
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return "", ErrUnauthorized
	}
	id := domain.MemberID(claims.Subject)
	if !a.couple.Contains(id) {
		return "", ErrUnauthorized
	}
	return id, nil
}

// Middleware は Authorization: Bearer トークンを検証し、
// AccountID をコンテキストに載せて次のハンドラへ渡す。
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "認証トークンが必要です")
			return
		}
		memberID, err := a.Verify(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "認証トークンが無効です")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), memberIDKey, memberID)))
	})
}
