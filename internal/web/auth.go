package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tacky0612/duo-pocketbook/internal/config"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ErrUnauthorized は認証失敗を表すエラー。
var ErrUnauthorized = errors.New("認証に失敗しました")

type contextKey string

const memberIDKey contextKey = "memberID"

// MemberIDFromContext はリクエストコンテキストから認証済みメンバーIDを取り出す。
func MemberIDFromContext(ctx context.Context) (domain.MemberID, bool) {
	id, ok := ctx.Value(memberIDKey).(domain.MemberID)
	return id, ok
}

// Authenticator は2メンバーのパスワード認証とJWTの発行・検証を担う。
type Authenticator struct {
	credentials map[domain.MemberID]config.MemberCredential
	couple      domain.Couple
	secret      []byte
	ttl         time.Duration
	now         func() time.Time
}

// NewAuthenticator は Authenticator を生成する。
func NewAuthenticator(cfg config.Config, couple domain.Couple, now func() time.Time) *Authenticator {
	if now == nil {
		now = time.Now
	}
	creds := map[domain.MemberID]config.MemberCredential{}
	for _, m := range cfg.Members {
		creds[m.Member.ID] = m
	}
	return &Authenticator{
		credentials: creds,
		couple:      couple,
		secret:      []byte(cfg.JWTSecret),
		ttl:         cfg.TokenTTL,
		now:         now,
	}
}

// Login はメンバーID・パスワードを検証してJWTを発行する。
func (a *Authenticator) Login(memberID domain.MemberID, password string) (token string, member domain.Member, expiresAt time.Time, err error) {
	cred, ok := a.credentials[memberID]
	if !ok || !cred.VerifyPassword(password) {
		return "", domain.Member{}, time.Time{}, ErrUnauthorized
	}

	expiresAt = a.now().Add(a.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   string(memberID),
		IssuedAt:  jwt.NewNumericDate(a.now()),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
	if err != nil {
		return "", domain.Member{}, time.Time{}, fmt.Errorf("トークンの発行に失敗しました: %w", err)
	}
	return token, cred.Member, expiresAt, nil
}

// Verify はJWTを検証してメンバーIDを返す。
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
// メンバーIDをコンテキストに載せて次のハンドラへ渡す。
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
