# 開発ガイド

## 必要ツール

| ツール | 用途 |
|---|---|
| Go 1.24+ | バックエンドのビルド・テスト |
| Node.js 22+ | フロントエンドのビルド |
| Docker（OrbStack / Docker Desktop） | ローカル環境・統合テスト |
| Terraform 1.5+ | インフラの検証・デプロイ |

## よく使うコマンド

```bash
make test                # ユニットテスト
make lint                # gofmt チェック + go vet
make up                  # ローカル環境起動（app + DynamoDB Local）→ http://localhost:8080
make test-integration    # 統合テスト（要 make up）
make down                # ローカル環境停止（データ削除）
make frontend            # フロントエンドのビルド（frontend/dist）
make build-lambda        # Lambdaデプロイパッケージ生成（build/lambda.zip）
make docs-validate       # docs/ 内の Mermaid 図の構文検証
```

単一テストの実行:

```bash
go test -run TestCalculateSettlement ./internal/domain/...
go test -run TestE2EFlow -tags=integration ./integration/...
```

## テスト戦略

### ユニットテスト（外部依存なし）

`go test ./...` で完結する。ドメイン層（精算計算のテーブルテスト）、アプリケーション層（`memory/` リポジトリを注入）、Web層（`httptest` でルーター全体を起動）を対象とする。

- 時刻は `now func() time.Time` を注入して固定する
- 認証は平文パスワード設定（`PasswordPlain`）を使いbcryptコストを回避する

### 統合テスト（ローカルに閉じたE2E）

`integration/` 配下、`//go:build integration` タグで分離。Docker Compose で起動した実構成（Goアプリ + DynamoDB Local）へ **HTTP経由** でアクセスし、ログイン→支出登録→収入入力→精算検証のフローを検証する。**実AWSなど外部への通信は行わない**。

```bash
make up && make test-integration
```

- テストデータの月は実行ごとにユニークな年月を使い、再実行時の干渉を避けている
- 接続先は `BASE_URL` 環境変数で変更可能（デフォルト `http://localhost:8080`）

## ローカル環境の構成

`docker-compose.yml`:

- `dynamodb` — `amazon/dynamodb-local`（インメモリ、停止でデータ消滅）
- `app` — マルチステージDockerfileでフロント（Vite）+ バックエンド（Go）をビルドした単一コンテナ。`/health` のヘルスチェック付き

テストアカウント（ローカル専用・平文）:

| メンバーID | パスワード | 表示名 |
|---|---|---|
| `taro` | `taro-password` | 太郎 |
| `hanako` | `hanako-password` | 花子 |

### Dockerを使わない最小起動（インメモリ）

```bash
MEMBER1_ID=taro MEMBER1_PASSWORD=pass1 \
MEMBER2_ID=hanako MEMBER2_PASSWORD=pass2 \
JWT_SECRET=dev-secret go run ./cmd/server
```

`TABLE_NAME` 未設定時はインメモリリポジトリで動作する（再起動でデータ消滅）。

### フロントエンドの開発

```bash
cd frontend && npm run dev        # Vite dev server
cd frontend && npm run typecheck  # tsc --noEmit（型チェックのみ）
cd frontend && npm run build      # tsc --noEmit → vite build
```

ログイン画面の「APIのURL」にAPIサーバー（例 `http://localhost:8080`）を入力する。値は localStorage に保存される。

フロントエンドは **TypeScript（strict）**。`tsconfig.json` で `strict: true` を有効にしており、`npm run build` はビルド前に `tsc --noEmit` で型チェックを行う（型エラーがあればビルドは失敗する）。CI（`frontend-build` ジョブ）でも `npm run build` を実行するため、型エラーは PR で検出される。共有ドメイン／API 型は `src/types.ts` に集約し、`api<T>()` の戻り値型として使う。

### デモモード（バックエンド不要）

ログイン画面の「デモモードで試す（API不要）」ボタンで、Lambda/API を用意せずに全機能を体験できる。API 通信をブラウザ内のモックへ委譲する仕組みで、GitHub Pages 配信物にもそのまま含まれる（デプロイの追加設定は不要）。

- 有効化フラグは `session.demo`（localStorage キー `demo`）。ボタン押下で `true` になり、ログアウトで解除される
- API 通信の唯一の集約点 `frontend/src/lib/apiClient.ts` の `api()` が、デモ時は `frontend/src/demo/` を**動的 import** してモックへ委譲する（本体とは別チャンクに分割され、デモを起動したときだけ読み込まれる）
- 編集データ（支出・収入・比重など）は localStorage キー `demo:db` に保存され、リロードしても維持される。設定画面の「デモデータをリセット」で初期状態へ戻せる

デモ用コードは `frontend/src/demo/` に集約している:

| ファイル | 責務 |
|---|---|
| `mockData.ts` | 初期シードデータ（実行時の直近3か月分を生成） |
| `store.ts` | インメモリ可変ストア（localStorage 永続化・リセット） |
| `settlement.ts` | 精算計算の TypeScript 移植（`internal/domain/settlement.go` と同一ロジック） |
| `demoApi.ts` | 各エンドポイントをモックへマッピングするルーター |
| `index.ts` | バレル（`apiClient` から動的 import される） |

デモの2アカウントは `taro`（太郎）/ `hanako`（花子）。デモモードではパスワードは検証しない。

## 環境変数

| 変数 | 必須 | 説明 |
|---|---|---|
| `MEMBER1_ID` / `MEMBER2_ID` | ✅ | メンバーID（ログインID） |
| `MEMBER1_NAME` / `MEMBER2_NAME` | | 表示名（省略時はID） |
| `MEMBERn_PASSWORD_HASH` | ※ | bcryptハッシュ（本番用。`go run ./cmd/hashpw` で生成） |
| `MEMBERn_PASSWORD` | ※ | 平文パスワード（ローカル専用）。※どちらか一方が必須 |
| `JWT_SECRET` | ✅ | JWT署名シークレット |
| `TOKEN_TTL_HOURS` | | トークン有効時間（デフォルト720=30日） |
| `TABLE_NAME` | | DynamoDBテーブル名。未設定ならインメモリ |
| `DYNAMO_ENDPOINT` | | DynamoDB Localのエンドポイント（設定時はテーブル自動作成） |
| `ALLOWED_ORIGINS` | | CORS許可オリジン（カンマ区切り、デフォルト`*`） |
| `PORT` / `STATIC_DIR` | | サーバーポート / 静的配信ディレクトリ（cmd/serverのみ） |

## CI（GitHub Actions）

`.github/workflows/ci.yml` がpush/PRで実行される:

1. **Lint** — gofmt / go vet / terraform fmt・validate
2. **Unit Test** — gotestsum でJUnit XML生成、カバレッジをStep Summaryへ
3. **Integration Test** — Docker Composeを起動して統合テスト
4. **Frontend Build** — Viteビルド
5. **Docs Mermaid Validation** — `docs/` 配下のMermaid図の構文検証（`make docs-validate` と同内容）

Unit / Integration のテストレポートは dorny/test-reporter により **PRのChecksタブ** に表示される。
