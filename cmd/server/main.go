// ローカル検証用のHTTPサーバー。frontend/ ディレクトリの静的配信も行う。
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/tacky0612/duo-pocketbook/internal/config"
	"github.com/tacky0612/duo-pocketbook/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定の読み込みに失敗", "error", err)
		os.Exit(1)
	}

	staticDir := cfg.StaticDir
	if staticDir == "" {
		// Vite でビルドされたフロントエンドがあれば配信する
		if _, err := os.Stat("frontend/dist"); err == nil {
			staticDir = "frontend/dist"
		}
	}

	handler, err := web.BuildHandler(context.Background(), cfg, web.RouterOption{StaticDir: staticDir})
	if err != nil {
		slog.Error("アプリケーションの初期化に失敗", "error", err)
		os.Exit(1)
	}

	addr := ":" + cfg.Port
	slog.Info("サーバーを起動します", "addr", addr, "staticDir", staticDir)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("サーバーが停止しました", "error", err)
		os.Exit(1)
	}
}
