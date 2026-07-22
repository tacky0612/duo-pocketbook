package domain

import "errors"

// ErrValidation は入力値がドメインの制約を満たさない場合のエラー。
var ErrValidation = errors.New("validation error")

// ErrIncomeNotReady は精算対象月の収入が揃っていない場合のエラー。
var ErrIncomeNotReady = errors.New("両メンバーの収入が入力されていません")
