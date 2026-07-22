package domain

import "fmt"

// Money は日本円の金額（整数円）を表す値オブジェクト。
// 負の値も許容する（精算計算の中間値などで利用する）。
type Money int64

// Add は金額を加算した新しい Money を返す。
func (m Money) Add(other Money) Money { return m + other }

// Sub は金額を減算した新しい Money を返す。
func (m Money) Sub(other Money) Money { return m - other }

// IsNegative は負の金額かどうかを返す。
func (m Money) IsNegative() bool { return m < 0 }

// String は "12345円" 形式の文字列を返す。
func (m Money) String() string { return fmt.Sprintf("%d円", int64(m)) }
