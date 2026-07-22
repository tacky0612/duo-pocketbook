package domain

import "fmt"

// Weight は精算の比重を表す値オブジェクト。
// 比重が大きいメンバーほど、精算後の可処分所得が多く残るように精算される。
// 例: 比重 1:1 なら可処分所得が等しくなり、2:1 なら前者が後者の2倍になる。
type Weight struct {
	weights map[MemberID]int64
}

// NewWeight は2人分の比重から Weight を生成する。比重は正の整数。
func NewWeight(idA MemberID, weightA int64, idB MemberID, weightB int64) (Weight, error) {
	if idA == "" || idB == "" || idA == idB {
		return Weight{}, fmt.Errorf("%w: 比重には異なる2人のメンバーIDが必要です", ErrValidation)
	}
	if weightA <= 0 || weightB <= 0 {
		return Weight{}, fmt.Errorf("%w: 比重は正の整数で指定してください: %d:%d", ErrValidation, weightA, weightB)
	}
	return Weight{weights: map[MemberID]int64{idA: weightA, idB: weightB}}, nil
}

// Of は指定メンバーの比重を返す。
func (w Weight) Of(id MemberID) (int64, bool) {
	v, ok := w.weights[id]
	return v, ok
}

// Entries はメンバーIDごとの比重のコピーを返す。
func (w Weight) Entries() map[MemberID]int64 {
	entries := make(map[MemberID]int64, len(w.weights))
	for id, v := range w.weights {
		entries[id] = v
	}
	return entries
}

// IsZero は未初期化かどうかを返す。
func (w Weight) IsZero() bool { return w.weights == nil }
