# duo-pocketbook

[![CI](https://github.com/tacky0612/duo-pocketbook/actions/workflows/ci.yml/badge.svg)](https://github.com/tacky0612/duo-pocketbook/actions/workflows/ci.yml)
[![Deploy to GitHub Pages](https://github.com/tacky0612/duo-pocketbook/actions/workflows/deploy-pages.yml/badge.svg)](https://github.com/tacky0612/duo-pocketbook/actions/workflows/deploy-pages.yml)

クライアント2人で使う家計簿Webアプリケーション。共有支出を2アカウントから登録し、月次で双方の収入を入力すると、指定した比重で双方の可処分所得が揃うように精算額（振込額）を算出します。

📚 **詳細ドキュメント**: [docs/](docs/README.md)（アーキテクチャ / 精算仕様 / API / データモデル / 開発 / デプロイ）

## 精算の仕組み

各メンバーの純額を `net = 収入 - 立替済み共有支出` とし、比重 `wA:wB` に対して

```
AからBへの振込額 t = (wB × netA - wA × netB) / (wA + wB)   ※端数は四捨五入
```

を計算します（`t` が負の場合はBからAへの振込）。精算後は `可処分所得A/wA == 可処分所得B/wB` が成り立ちます。

> 例: 比重1:1、夫（収入10万・支出2万）、妻（収入5万・支出2万）
> → 夫が妻に **2.5万円** 振り込むと、双方の可処分所得が5.5万円で等しくなります。

## アーキテクチャ

クリーンアーキテクチャ + ドメイン駆動設計。依存の向きは常に内側（domain）へ。

| レイヤー | パス | 責務 |
|---|---|---|
| ドメイン層 | `internal/domain/` | 精算計算などの重要判断。**外部依存ゼロ**（標準ライブラリのみ） |
| アプリケーション層 | `internal/application/` | ユースケース。リポジトリはインターフェイス経由 |
| インフラ層 | `internal/infrastructure/` | DynamoDB / インメモリのリポジトリ実装 |
| Web層 | `internal/web/` | APIインターフェイス（ハンドラ・ルーティング・JWT認証・CORS） |

- **API仕様**: [`api/openapi.yaml`](api/openapi.yaml)（OpenAPI 3.0）
- **フロントエンド**: `frontend/`（TypeScript + React + Vite + Tailwind CSS、GitHub Pages配信）
- **インフラ**: `terraform/`（Lambda Function URL + DynamoDB。**AWS無料枠のみ**を使用）

## ローカル開発

必要ツール: Go 1.24+ / Node.js 22+ / Docker（OrbStackなど）/ Terraform

```bash
# ユニットテスト
make test

# ローカル環境の起動（アプリ + DynamoDB Local、外部通信なし）
make up            # → http://localhost:8080 （taro / taro-password でログイン）

# 統合テスト（要 make up）
make test-integration

# 停止
make down

# フロントエンドのみ開発する場合（Vite dev server）
cd frontend && npm run dev
```

DynamoDBなしで手早くAPIを動かす場合（インメモリ、再起動でデータ消滅）:

```bash
MEMBER1_ID=taro MEMBER1_PASSWORD=pass1 \
MEMBER2_ID=hanako MEMBER2_PASSWORD=pass2 \
JWT_SECRET=dev-secret go run ./cmd/server
```

## デプロイ

### API（AWS: 無料枠のみ）

利用リソースはすべて常時無料枠内:

- **Lambda** (128MB/arm64) + **Function URL** — 100万リクエスト/月まで永続無料
- **DynamoDB** — PROVISIONED 1RCU/1WCU（常時無料枠25以内）
- API Gatewayは12ヶ月無料のみのため**不使用**

```bash
# 1. パスワードハッシュを生成し terraform/terraform.tfvars を作成
go run ./cmd/hashpw 'your-password'
cp terraform/terraform.tfvars.example terraform/terraform.tfvars  # 値を編集

# 2. Lambdaパッケージをビルドしてデプロイ
make build-lambda
terraform -chdir=terraform init
terraform -chdir=terraform apply   # 出力の function_url がAPIエンドポイント
```

### フロントエンド（GitHub Pages）

`main` ブランチの `frontend/` 変更をpushすると、GitHub Actions（`deploy-pages.yml`）が自動でビルド・デプロイします。リポジトリ設定で Pages のソースを「GitHub Actions」にしてください。

デプロイ後、ログイン画面の「APIのURL」に Function URL を入力して利用します。`terraform.tfvars` の `allowed_origins` に Pages のURLを設定してください。

## CI

GitHub Actions（`.github/workflows/ci.yml`）で以下を実行し、テストレポートをPRのChecksに表示します:

1. **Lint** — gofmt / go vet / terraform fmt・validate
2. **Unit Test** — `go test ./...` + カバレッジサマリ
3. **Integration Test** — Docker Composeでローカル環境を起動しE2Eテスト（外部通信なし）
4. **Frontend Build** — Viteビルド
