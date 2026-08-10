package domain

import "errors"

// ErrValidation は入力値がドメインの制約を満たさない場合のエラー。
var ErrValidation = errors.New("validation error")

// ErrIncomeNotReady は精算対象月の給与が揃っていない場合のエラー。
var ErrIncomeNotReady = errors.New("両メンバーの給与が入力されていません")

// ErrSettled は精算確定済み（スナップショット保存済み）の月に属するデータを
// 変更・削除しようとした場合のエラー。確定済みの月は精算を取り消してから編集する。
var ErrSettled = errors.New("精算確定済みの月は編集できません")
