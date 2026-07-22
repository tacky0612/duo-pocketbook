// hashpw はパスワードの bcrypt ハッシュを生成する補助ツール。
// Terraform 変数 (memberN_password_hash) の値の生成に使う。
//
// 使い方: go run ./cmd/hashpw 'your-password'
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "使い方: go run ./cmd/hashpw '<password>'")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ハッシュ生成に失敗:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
