package application

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

const maxMemberNameLen = 20

// defaultMemberColors はカラー未設定時のデフォルト（メンバーの並び順に対応）。
var defaultMemberColors = [2]string{"#2563eb", "#4f46e5"}

var hexColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// MemberView は表示名・カラーを反映したメンバー情報（API向け）。
type MemberView struct {
	ID    domain.MemberID
	Name  string
	Color string
}

// SettingsUsecase は精算比重の設定に関するユースケース。
type SettingsUsecase struct {
	couple   domain.Couple
	settings SettingsRepository
}

// NewSettingsUsecase は SettingsUsecase を生成する。
func NewSettingsUsecase(couple domain.Couple, settings SettingsRepository) *SettingsUsecase {
	return &SettingsUsecase{couple: couple, settings: settings}
}

// GetWeight は現在の精算比重を返す。未設定の場合はデフォルトの1:1を返す。
func (u *SettingsUsecase) GetWeight(ctx context.Context) (domain.Weight, error) {
	return currentWeight(ctx, u.settings, u.couple)
}

// GetMembers はプロフィール（表示名・カラー）を反映したメンバー一覧を返す。
func (u *SettingsUsecase) GetMembers(ctx context.Context) ([2]MemberView, error) {
	profiles, err := u.settings.GetMemberProfiles(ctx)
	if err != nil {
		return [2]MemberView{}, fmt.Errorf("プロフィールの取得に失敗しました: %w", err)
	}
	base := u.couple.Members()
	var views [2]MemberView
	for i, m := range base {
		views[i] = memberView(m, i, profiles[m.ID])
	}
	return views, nil
}

// GetMember は1人分のプロフィール反映済みメンバー情報を返す。
func (u *SettingsUsecase) GetMember(ctx context.Context, id domain.MemberID) (MemberView, error) {
	members, err := u.GetMembers(ctx)
	if err != nil {
		return MemberView{}, err
	}
	for _, v := range members {
		if v.ID == id {
			return v, nil
		}
	}
	return MemberView{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, id)
}

// memberView はベースのメンバー・並び順・プロフィール上書きから表示用の値を組み立てる。
func memberView(base domain.Member, index int, p MemberProfile) MemberView {
	name := base.Name
	if p.Name != "" {
		name = p.Name
	}
	color := defaultMemberColors[index%len(defaultMemberColors)]
	if p.Color != "" {
		color = p.Color
	}
	return MemberView{ID: base.ID, Name: name, Color: color}
}

// UpdateMemberName はメンバーの表示名を更新する。
func (u *SettingsUsecase) UpdateMemberName(ctx context.Context, id domain.MemberID, name string) error {
	if !u.couple.Contains(id) {
		return fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, id)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: 表示名は必須です", domain.ErrValidation)
	}
	if len([]rune(name)) > maxMemberNameLen {
		return fmt.Errorf("%w: 表示名は%d文字以内で指定してください", domain.ErrValidation, maxMemberNameLen)
	}
	if err := u.settings.SaveMemberName(ctx, id, name); err != nil {
		return fmt.Errorf("表示名の保存に失敗しました: %w", err)
	}
	return nil
}

// UpdateMemberColor はメンバーのカラーを更新する（"#RRGGBB" 形式）。
func (u *SettingsUsecase) UpdateMemberColor(ctx context.Context, id domain.MemberID, color string) error {
	if !u.couple.Contains(id) {
		return fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, id)
	}
	color = strings.TrimSpace(color)
	if !hexColorRe.MatchString(color) {
		return fmt.Errorf("%w: カラーは #RRGGBB 形式で指定してください: %q", domain.ErrValidation, color)
	}
	if err := u.settings.SaveMemberColor(ctx, id, strings.ToLower(color)); err != nil {
		return fmt.Errorf("カラーの保存に失敗しました: %w", err)
	}
	return nil
}

// GetClosingDay は現在の締め日を返す。未設定の場合はデフォルト（暦月どおり）を返す。
func (u *SettingsUsecase) GetClosingDay(ctx context.Context) (domain.ClosingDay, error) {
	return currentClosingDay(ctx, u.settings)
}

// UpdateClosingDay は締め日（1〜31）を更新する。
func (u *SettingsUsecase) UpdateClosingDay(ctx context.Context, day int) (domain.ClosingDay, error) {
	cd, err := domain.NewClosingDay(day)
	if err != nil {
		return 0, err
	}
	if err := u.settings.SaveClosingDay(ctx, cd); err != nil {
		return 0, fmt.Errorf("締め日の保存に失敗しました: %w", err)
	}
	return cd, nil
}

// currentClosingDay は設定済みの締め日、未設定ならデフォルト（暦月どおり）を返す。
func currentClosingDay(ctx context.Context, settings SettingsRepository) (domain.ClosingDay, error) {
	cd, ok, err := settings.GetClosingDay(ctx)
	if err != nil {
		return 0, fmt.Errorf("締め日の取得に失敗しました: %w", err)
	}
	if ok {
		return cd, nil
	}
	return domain.DefaultClosingDay, nil
}

// UpdateWeightInput は比重更新の入力。メンバーIDごとの比重を指定する。
type UpdateWeightInput struct {
	Weights map[domain.MemberID]int64
}

// UpdateWeight は精算比重を更新する。
func (u *SettingsUsecase) UpdateWeight(ctx context.Context, in UpdateWeightInput) (domain.Weight, error) {
	members := u.couple.Members()
	for id := range in.Weights {
		if !u.couple.Contains(id) {
			return domain.Weight{}, fmt.Errorf("%w: 不明なメンバーです: %s", domain.ErrValidation, id)
		}
	}
	weight, err := domain.NewWeight(
		members[0].ID, in.Weights[members[0].ID],
		members[1].ID, in.Weights[members[1].ID],
	)
	if err != nil {
		return domain.Weight{}, err
	}
	if err := u.settings.SaveWeight(ctx, weight); err != nil {
		return domain.Weight{}, fmt.Errorf("比重の保存に失敗しました: %w", err)
	}
	return weight, nil
}

// effectiveCouple は表示名の上書き設定を反映した Couple を返す。
func effectiveCouple(ctx context.Context, settings SettingsRepository, base domain.Couple) (domain.Couple, error) {
	profiles, err := settings.GetMemberProfiles(ctx)
	if err != nil {
		return domain.Couple{}, fmt.Errorf("プロフィールの取得に失敗しました: %w", err)
	}
	members := base.Members()
	for i := range members {
		if p, ok := profiles[members[i].ID]; ok && p.Name != "" {
			members[i].Name = p.Name
		}
	}
	return domain.NewCouple(members[0], members[1])
}

// currentWeight は設定済みの比重、未設定ならデフォルトの1:1を返す。
func currentWeight(ctx context.Context, settings SettingsRepository, couple domain.Couple) (domain.Weight, error) {
	weight, ok, err := settings.GetWeight(ctx)
	if err != nil {
		return domain.Weight{}, fmt.Errorf("比重の取得に失敗しました: %w", err)
	}
	if ok {
		return weight, nil
	}
	members := couple.Members()
	return domain.NewWeight(members[0].ID, 1, members[1].ID, 1)
}
