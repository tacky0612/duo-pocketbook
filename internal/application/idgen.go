package application

import (
	"crypto/rand"
	"encoding/hex"
)

// newIDSuffix は支出IDのサフィックスとして使うランダムな16進文字列を返す。
func newIDSuffix() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand の失敗は実行環境の異常であり回復不能。
		panic(err)
	}
	return hex.EncodeToString(b)
}
