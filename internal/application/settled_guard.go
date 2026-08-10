package application

import (
	"context"
	"fmt"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ensureMonthNotSettled は対象月が精算確定済み（スナップショットが存在する）なら
// domain.ErrSettled を返す。確定済みの月に属するデータの変更・削除を防ぐガードに使う。
//
// 特定月に紐づかない項目（毎月継続の収入・立替精算、固定費）は対象月が定まらないため、
// このガードの呼び出し側で対象外とする（呼び出さない）。
func ensureMonthNotSettled(ctx context.Context, snapshots SettlementSnapshotRepository, ym domain.YearMonth) error {
	_, ok, err := snapshots.Find(ctx, ym)
	if err != nil {
		return fmt.Errorf("精算スナップショットの取得に失敗しました: %w", err)
	}
	if ok {
		return fmt.Errorf("%w: %s", domain.ErrSettled, ym)
	}
	return nil
}
