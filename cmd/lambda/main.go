// AWS Lambda (Function URL) 用エントリポイント。
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/tacky0612/duo-pocketbook/internal/config"
	"github.com/tacky0612/duo-pocketbook/internal/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定の読み込みに失敗", "error", err)
		os.Exit(1)
	}
	handler, err := web.BuildHandler(context.Background(), cfg, web.RouterOption{})
	if err != nil {
		slog.Error("アプリケーションの初期化に失敗", "error", err)
		os.Exit(1)
	}
	// Lambda Function URL はペイロード形式 2.0 (API Gateway v2 互換)。
	lambda.Start(httpadapter.NewV2(handler).ProxyWithContext)
}
