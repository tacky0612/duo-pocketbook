# コントリビュートガイド

duo-pocketbook への貢献を歓迎します。バグ報告・機能提案・Pull Request のいずれも大歓迎です。このドキュメントは開発の進め方をまとめたものです。

## はじめに

- **バグ報告・機能提案**: [Issue](https://github.com/tacky0612/duo-pocketbook/issues) を作成してください。
- **脆弱性の報告**: 公開 Issue では**なく**、[SECURITY.md](SECURITY.md) の手順（GitHub の Private vulnerability reporting）に従ってください。
- 大きめの変更を送る前に、まず Issue で方針を相談すると手戻りが減ります。
- すべてのやり取りは [行動規範（CODE_OF_CONDUCT.md）](CODE_OF_CONDUCT.md) に従ってください。

## 必要ツール

- **Go**（`go.mod` に記載のバージョン以上）
- **Node.js 22+**（フロントエンド）
- **Docker**（OrbStack など。統合テストとローカル環境で使用）
- **Terraform**（インフラを触る場合のみ）

## 開発フロー

1. リポジトリを fork し、ブランチを切る（例: `feat/xxx` / `fix/xxx`）。
2. 変更を加える。
3. 下記のチェックをローカルで通す。
4. Pull Request を作成する（base は `main`）。CI（Lint / UnitTest / IntegrationTest / Frontend Build / OpenAPI 同期 / CodeQL）が通ることを確認してください。

### よく使うコマンド

```bash
make test                # ユニットテスト（外部依存なし）
make lint                # gofmt チェック + go vet
make fmt                 # gofmt -w + terraform fmt（整形）
make up                  # ローカル環境起動（アプリ + DynamoDB Local）→ http://localhost:8080
make test-integration    # 統合テスト（要 make up）
make down                # ローカル環境停止
make frontend            # フロントエンドのビルド（frontend/dist）
make openapi             # Goコードの注釈から api/openapi.yaml を生成
make openapi-check       # openapi.yaml がコードと同期しているか検証（CI と同じ）
make docs-validate       # docs 内の Mermaid 図の構文検証

cd frontend && npm run dev   # フロント開発サーバー
```

ローカルのテストアカウント: `taro` / `taro-password`、`hanako` / `hanako-password`（`docker-compose.yml` で定義）。

## コーディング規約

- **Go**: `gofmt` 準拠（`make fmt` で整形、`make lint` で検証）。ドメイン層（`internal/domain/`）は**外部依存を一切 import しない**（クリーンアーキテクチャ + DDD。依存の向きは常に内側へ）。
- **TypeScript / React**: strict モードで型付けし、UI はコンポーネントで組み立てる。**CSS は書かず Tailwind ユーティリティ**を使う。共有ドメイン／API 型は `frontend/src/types.ts` に集約する。
- **インフラ**: AWS は**無料枠のリソースのみ**を使用する（DynamoDB は PROVISIONED 1RCU/1WCU、API Gateway 不使用など。詳細は [docs/deployment.md](docs/deployment.md)）。
- コメント・命名は周囲のコードのスタイルに合わせる。

## API 定義（重要）

`api/openapi.yaml` は **`internal/web` のハンドラ swag 注釈・DTO 型が正（source of truth）**で、`make openapi` により自動生成します。**手で編集しないでください。** API を変更したらハンドラの注釈・DTO を更新し、`make openapi` で再生成してください。CI の `openapi-check` が未再生成を検出して失敗します。

## ドキュメント

コード・API・インフラ・開発手順を変更したら、`docs/` 配下の対応するドキュメントも同期更新してください（[docs/README.md](docs/README.md) が目次）。図はすべて **Mermaid 記法**で書き、`make docs-validate` で検証します。実装と乖離した記述・架空のコマンドは残さないでください。

## テスト方針

- **UnitTest**: ドメイン層・アプリケーション層・Web 層を対象。外部依存なしで `go test ./...` が通る状態を保つ（リポジトリは `internal/infrastructure/memory` 実装を使う）。
- **IntegrationTest**（`integration/`）: `//go:build integration` タグで分離。Docker Compose のローカル環境（アプリ + DynamoDB Local）に HTTP でアクセスする。**実 AWS 等の外部への通信は行わない。**
- 精算計算（`internal/domain/settlement.go`）などのドメインロジックはユニットテストで網羅してください。

## Pull Request のチェックリスト

- [ ] `make lint` / `make test` が通る
- [ ] API を変更した場合は `make openapi` を実行し、生成物をコミットした
- [ ] 関連ドキュメント（`docs/`）を更新した
- [ ] 変更内容と目的が PR 本文から分かる

ご協力ありがとうございます 🙌
