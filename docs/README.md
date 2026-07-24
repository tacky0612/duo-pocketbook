# duo-pocketbook ドキュメント

クライアント2人で使う家計簿Webアプリケーション。共有支出を2アカウントから登録し、月次で双方の収入を入力すると、指定した比重で双方の可処分所得が揃うように精算額を算出する。

## 目次

| ドキュメント | 内容 |
|---|---|
| [architecture.md](architecture.md) | クリーンアーキテクチャ + DDD のレイヤー構成と依存関係 |
| [infrastructure.md](infrastructure.md) | AWS・Cloudflare・GitHub のインフラ構成図（draw.io 併置）と各リソースの説明 |
| [settlement.md](settlement.md) | 精算計算の仕様（計算式・端数処理・具体例） |
| [api.md](api.md) | API利用ガイド（認証フロー・エンドポイント・エラー） |
| [data-model.md](data-model.md) | DynamoDB シングルテーブル設計 |
| [development.md](development.md) | ローカル開発・テストの手順 |
| [deployment.md](deployment.md) | AWS / GitHub Pages へのデプロイ手順と無料枠の制約 |
| [secrets.md](secrets.md) | 運用に必要なシークレット／変数の一覧と管理方法 |

## 技術スタック

- **バックエンド**: Go（標準 `net/http`、外部フレームワーク不使用）
- **フロントエンド**: TypeScript + React + Vite + Tailwind CSS（`frontend/`）
- **データストア**: Amazon DynamoDB（ローカルは DynamoDB Local）
- **実行基盤**: AWS Lambda + Function URL（**AWS無料枠のみ**で運用）
- **IaC**: Terraform（`terraform/`）
- **API仕様**: OpenAPI 3.0（[`api/openapi.yaml`](../api/openapi.yaml)）
- **CI/CD**: GitHub Actions（テスト・レポート、GitHub Pages デプロイ）
