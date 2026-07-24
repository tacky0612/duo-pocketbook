package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

const (
	minPasswordLen = 8
	maxLoginIDLen  = 32
)

var loginIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ErrUnauthorized はログイン認証に失敗したことを表す。
var ErrUnauthorized = errors.New("authentication failed")

// AccountSeed はアカウント初回作成時の初期値（env 由来）。
type AccountSeed struct {
	LoginID string // 初期ログインID（env の ACCOUNTn_LOGINID）
	Name    string // 表示名の初期値
	Hash    string // bcrypt ハッシュ（本番）
	Plain   string // 平文パスワード（ローカル。Hash 未設定時に使う）
}

// AccountUsecase はアカウント（不変の AccountID・可変のログインID/パスワード）を扱う。
type AccountUsecase struct {
	repo  AccountRepository
	seeds [2]AccountSeed
	idgen func() string
}

// NewAccountUsecase は AccountUsecase を生成する。idgen が nil の場合はランダム生成を使う。
func NewAccountUsecase(repo AccountRepository, seeds [2]AccountSeed, idgen func() string) *AccountUsecase {
	if idgen == nil {
		idgen = generateAccountID
	}
	return &AccountUsecase{repo: repo, seeds: seeds, idgen: idgen}
}

// generateAccountID は不変の AccountID（acct_<hex>）を生成する。
func generateAccountID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "acct_" + hex.EncodeToString(b)
}

// Provision は2スロット分のアカウントを保証し（無ければ生成して永続化）、
// 各アカウントの AccountID と表示名から成る Member を返す。
func (u *AccountUsecase) Provision(ctx context.Context) ([2]domain.Member, error) {
	existing, err := u.repo.List(ctx)
	if err != nil {
		return [2]domain.Member{}, fmt.Errorf("アカウントの取得に失敗しました: %w", err)
	}
	bySlot := map[int]Account{}
	for _, a := range existing {
		bySlot[a.Slot] = a
	}

	var members [2]domain.Member
	for i := 0; i < 2; i++ {
		acc, ok := bySlot[i]
		if !ok {
			hash := u.seeds[i].Hash
			if hash == "" {
				h, err := bcrypt.GenerateFromPassword([]byte(u.seeds[i].Plain), bcrypt.DefaultCost)
				if err != nil {
					return [2]domain.Member{}, fmt.Errorf("パスワードハッシュの生成に失敗しました: %w", err)
				}
				hash = string(h)
			}
			acc = Account{
				ID:           domain.MemberID(u.idgen()),
				Slot:         i,
				LoginID:      u.seeds[i].LoginID,
				PasswordHash: hash,
			}
			if err := u.repo.Save(ctx, acc); err != nil {
				return [2]domain.Member{}, fmt.Errorf("アカウントの作成に失敗しました: %w", err)
			}
		}
		members[i] = domain.Member{ID: acc.ID, Name: u.seeds[i].Name}
	}
	return members, nil
}

// Authenticate はログインID・パスワードを検証し、一致すれば AccountID を返す。
func (u *AccountUsecase) Authenticate(ctx context.Context, loginID, password string) (domain.MemberID, error) {
	accounts, err := u.repo.List(ctx)
	if err != nil {
		return "", err
	}
	for _, a := range accounts {
		if a.LoginID == loginID {
			if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) == nil {
				return a.ID, nil
			}
			return "", ErrUnauthorized
		}
	}
	return "", ErrUnauthorized
}

// Get は AccountID からアカウントを返す。
func (u *AccountUsecase) Get(ctx context.Context, accountID domain.MemberID) (Account, error) {
	accounts, err := u.repo.List(ctx)
	if err != nil {
		return Account{}, err
	}
	for _, a := range accounts {
		if a.ID == accountID {
			return a, nil
		}
	}
	return Account{}, ErrNotFound
}

// UpdateLoginID はログインIDを更新する（空・形式・重複を検証）。
func (u *AccountUsecase) UpdateLoginID(ctx context.Context, accountID domain.MemberID, newLoginID string) error {
	newLoginID = strings.TrimSpace(newLoginID)
	if newLoginID == "" || len(newLoginID) > maxLoginIDLen || !loginIDRe.MatchString(newLoginID) {
		return fmt.Errorf("%w: ログインIDは英数字と . _ - のみ・%d文字以内で指定してください", domain.ErrValidation, maxLoginIDLen)
	}
	accounts, err := u.repo.List(ctx)
	if err != nil {
		return err
	}
	var target *Account
	for i := range accounts {
		if accounts[i].ID == accountID {
			target = &accounts[i]
		} else if accounts[i].LoginID == newLoginID {
			return fmt.Errorf("%w: そのログインIDは使用されています", domain.ErrValidation)
		}
	}
	if target == nil {
		return ErrNotFound
	}
	target.LoginID = newLoginID
	if err := u.repo.Save(ctx, *target); err != nil {
		return fmt.Errorf("ログインIDの保存に失敗しました: %w", err)
	}
	return nil
}

// UpdatePassword は現在のパスワードを検証したうえで新しいパスワードに更新する。
func (u *AccountUsecase) UpdatePassword(ctx context.Context, accountID domain.MemberID, current, newPassword string) error {
	if len([]rune(newPassword)) < minPasswordLen {
		return fmt.Errorf("%w: パスワードは%d文字以上で指定してください", domain.ErrValidation, minPasswordLen)
	}
	acc, err := u.Get(ctx, accountID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(acc.PasswordHash), []byte(current)) != nil {
		return fmt.Errorf("%w: 現在のパスワードが違います", domain.ErrValidation)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("パスワードハッシュの生成に失敗しました: %w", err)
	}
	acc.PasswordHash = string(h)
	if err := u.repo.Save(ctx, acc); err != nil {
		return fmt.Errorf("パスワードの保存に失敗しました: %w", err)
	}
	return nil
}
