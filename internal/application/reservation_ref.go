package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// liveReservationRef は実データ（支出・収入）の予約への参照を、予約がまだ存在する場合だけそのまま返す。
// 予約が削除済みならゼロ値（予約に由来しない）を返し、編集時に紐づけを外せるようにする。
func liveReservationRef(ctx context.Context, reservations ReservationRepository, ref domain.ReservationRef) (domain.ReservationRef, error) {
	id, ok := ref.ID()
	if !ok {
		return ref, nil
	}
	if _, err := reservations.FindByID(ctx, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return domain.ReservationRef{}, nil
		}
		return domain.ReservationRef{}, fmt.Errorf("予約の取得に失敗しました: %w", err)
	}
	return ref, nil
}
