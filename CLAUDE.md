# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## プロジェクト概要

duo-pocketbook はクライアント2人で使う家計簿 Web アプリケーション。2アカウントのみからのアクセスを想定し、共有支出を登録し、月次で双方の収入を入力して精算額を算出する。

**コアドメインロジック（精算計算）**: 指定した比重で双方の可処分所得が等しくなるよう振込額を計算する。
例: 比重1:1、夫の収入10万・支出2万、妻の収入5万・支出2万 → 可処分所得を等しくするため夫が妻に2.5万振り込む。この計算はドメイン層に外部依存なしで実装し、ユニットテストで網羅する。

## 技術スタックと制約

- **言語**: Go（バックエンド）、TypeScript + React + Vite + Tailwind CSS（`frontend/`。strict モードで型付けし、UIはコンポーネントで組み立てる。CSSは書かずTailwindユーティリティを使う。共有ドメイン／API 型は `frontend/src/types.ts` に集約）
- **インフラ**: AWS サーバーレス（Lambda Function URL + DynamoDB）。**必ず無料枠のリソースのみを利用する**
  - DynamoDB は PROVISIONED 1RCU/1WCU（常時無料枠25以内）。PAY_PER_REQUEST は無料枠対象外のため使用しない
  - API Gateway は12ヶ月無料のみのため使用しない（Function URL は永続無料）
- フロントエンド配信は GitHub Pages（`deploy-pages.yml` で自動デプロイ）
- **認証**: 環境変数設定の2ユーザー（bcryptハッシュ）+ JWT。Cognito不使用
- **IaC**: Terraform（`terraform/` 配下）
- **API 仕様**: `api/openapi.yaml`（OpenAPI 3.0）が外部ツール連携の契約。API 変更時は必ず更新する

## アーキテクチャ（クリーンアーキテクチャ + DDD）

依存の向きは常に内側（domain）へ。ディレクトリはレイヤーを反映する:

- **domain 層** (`internal/domain/`): エンティティ・値オブジェクト・ドメインサービス。精算計算（`settlement.go` の `CalculateSettlement`）などの重要判断をここに定義し、**外部依存（AWS SDK、DB、HTTP など）を一切 import しない**
- **application 層** (`internal/application/`): ユースケース。リポジトリインターフェイス（`repository.go`）経由で永続化にアクセス
- **infrastructure 層** (`internal/infrastructure/`): `dynamodb/`（本番・統合テスト）と `memory/`（ユニットテスト・軽量ローカル）のリポジトリ実装
- **web 層** (`internal/web/`): API インターフェイス（ハンドラ、ルーティング、JWT認証、CORS）。OpenAPI 定義と一致させる
- エントリポイント: `cmd/server`（ローカルHTTP + 静的配信）と `cmd/lambda`（Function URL）は同じルーター（`web.BuildHandler`）を共有する
- DynamoDB はシングルテーブル設計。支出IDは `<yyyy-MM>_<hex>` 形式で対象月を内包し、IDだけでパーティションを特定できる

## ドキュメント整備

`docs/` 配下にドキュメントを整備している。**コード・API・インフラ・開発手順を変更したら、対応するドキュメントを必ず同期更新する**こと。ドキュメントの構成・更新手順・記述ルールは `/docs` スキル（`.claude/skills/docs/SKILL.md`）に従う。

- `docs/README.md`（目次） / `architecture.md` / `settlement.md` / `api.md` / `data-model.md` / `development.md` / `deployment.md`
- 実装と乖離した記述を残さない。架空のコマンド・機能を書かない
- API変更時は `api/openapi.yaml` と `docs/api.md` の両方を更新する
- **図はすべて Mermaid 記法で書く**。図を追加・変更したら `make docs-validate` で構文を検証する（CIの `docs-mermaid` ジョブでも実行される）

## テスト方針

- **UnitTest**: ドメイン層・アプリケーション層・Web層を対象。外部依存なしで `go test ./...` で実行可能に保つ（リポジトリは `memory/` 実装を使う）
- **IntegrationTest** (`integration/`): `//go:build integration` タグで分離。Docker Compose のローカル環境（アプリ + DynamoDB Local）に HTTP でアクセスする。**外部（実 AWS 等）への通信は行わない**
- CI（`.github/workflows/ci.yml`）は Lint / UnitTest / IntegrationTest / フロントビルドを実行し、gotestsum の JUnit XML を dorny/test-reporter で **PR の Checks にテストレポート表示**、カバレッジは Step Summary に出力する

## よく使うコマンド

```bash
make test                               # ユニットテスト
go test -run TestName ./internal/domain/...  # 単一テスト
make up                                 # ローカル環境起動（app + DynamoDB Local）
make test-integration                   # 統合テスト（要 make up）
make down                               # ローカル環境停止
make lint                               # gofmt チェック + go vet
make frontend                           # フロントエンドのビルド（frontend/dist）
make build-lambda                       # Lambda デプロイパッケージ生成（build/lambda.zip）
terraform -chdir=terraform plan         # インフラ変更確認
go run ./cmd/hashpw '<password>'        # bcrypt ハッシュ生成（Terraform変数用）
cd frontend && npm run dev              # フロント開発サーバー
```

ローカル環境のテストアカウント: `taro`/`taro-password`, `hanako`/`hanako-password`（docker-compose.yml で定義）。
