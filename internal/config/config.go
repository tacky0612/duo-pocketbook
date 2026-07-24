// Package config は環境変数からアプリケーション設定を読み込む。
package config

import (
	"crypto/subtle"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// MemberCredential はメンバーと認証情報の組。
type MemberCredential struct {
	Member domain.Member
	// PasswordHash は bcrypt ハッシュ。本番環境ではこちらを設定する。
	PasswordHash string
	// PasswordPlain は平文パスワード。ローカル開発・テスト用。
	PasswordPlain string
}

// VerifyPassword はパスワードを検証する。
// PasswordHash（bcrypt）が設定されていればそちらを優先し、
// なければ PasswordPlain と定数時間比較する。
func (c MemberCredential) VerifyPassword(password string) bool {
	if c.PasswordHash != "" {
		return bcrypt.CompareHashAndPassword([]byte(c.PasswordHash), []byte(password)) == nil
	}
	if c.PasswordPlain == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.PasswordPlain), []byte(password)) == 1
}

// Config はアプリケーション全体の設定。
type Config struct {
	Members        [2]MemberCredential
	JWTSecret      string
	TokenTTL       time.Duration
	TableName      string
	DynamoEndpoint string
	Port           string
	AllowedOrigins []string
	StaticDir      string
	// ClientKey は正規クライアントだけを通すための事前共有キー。
	// 設定時は X-Client-Key ヘッダが一致しないリクエストを 403 で弾く（/health と CORS プリフライトは対象外）。
	// 空文字なら無効（後方互換・ローカル開発）。公開SPAに埋め込むため「秘密」ではなく、botノイズ低減の多層防御の一枚。
	ClientKey string
}

// Couple は設定された2メンバーから Couple を構築する。
func (c Config) Couple() (domain.Couple, error) {
	return domain.NewCouple(c.Members[0].Member, c.Members[1].Member)
}

// defaultMemberNames はメンバーの表示名の既定値。
// 表示名は変数/シークレットで持たず、ここを初期値としてアプリ側で設定する
// （環境変数 MEMBERn_NAME が設定されていればそちらを優先。実行時に画面から変更も可能）。
var defaultMemberNames = [2]string{"太郎", "花子"}

// Load は環境変数から設定を読み込む。
func Load() (Config, error) {
	var cfg Config
	for i := range cfg.Members {
		prefix := fmt.Sprintf("MEMBER%d_", i+1)
		id := os.Getenv(prefix + "ID")
		if id == "" {
			return Config{}, fmt.Errorf("環境変数 %sID は必須です", prefix)
		}
		name := os.Getenv(prefix + "NAME")
		if name == "" {
			name = defaultMemberNames[i]
		}
		hash := os.Getenv(prefix + "PASSWORD_HASH")
		plain := os.Getenv(prefix + "PASSWORD")
		if hash == "" && plain == "" {
			return Config{}, fmt.Errorf("環境変数 %sPASSWORD_HASH または %sPASSWORD のいずれかは必須です", prefix, prefix)
		}
		cfg.Members[i] = MemberCredential{
			Member:        domain.Member{ID: domain.MemberID(id), Name: name},
			PasswordHash:  hash,
			PasswordPlain: plain,
		}
	}

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("環境変数 JWT_SECRET は必須です")
	}

	cfg.TokenTTL = 30 * 24 * time.Hour
	if v := os.Getenv("TOKEN_TTL_HOURS"); v != "" {
		hours, err := strconv.Atoi(v)
		if err != nil || hours <= 0 {
			return Config{}, fmt.Errorf("環境変数 TOKEN_TTL_HOURS が不正です: %q", v)
		}
		cfg.TokenTTL = time.Duration(hours) * time.Hour
	}

	cfg.TableName = os.Getenv("TABLE_NAME")
	cfg.DynamoEndpoint = os.Getenv("DYNAMO_ENDPOINT")

	cfg.Port = os.Getenv("PORT")
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		origins = "*"
	}
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.AllowedOrigins = append(cfg.AllowedOrigins, o)
		}
	}

	cfg.StaticDir = os.Getenv("STATIC_DIR")
	cfg.ClientKey = os.Getenv("CLIENT_KEY")
	return cfg, nil
}
