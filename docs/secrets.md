# シークレット一覧

このリポジトリを本番運用（Terraform で AWS + Cloudflare をデプロイ）するために必要な機密情報の一覧。

## 原則
- **平文でコミットしない**。本番のシークレットは **GitHub Secrets / Variables** で管理し、CI（`terraform.yml`）が `TF_VAR_*` 等で注入する。詳細は [deployment.md](./deployment.md) の「CI/CD とシークレット管理」を参照。
- **tfstate にも機密が平文で入る**ため、state は暗号化・非公開の S3 リモートバックエンドに置く（`versions.tf` の `backend "s3"`）。
- **ローカル開発は専用の平文値**（`docker-compose.yml` にコミット済み）を使い、本番シークレットは不要。
- AWS は **OIDC**（長期アクセスキーを保存しない）。

## 1. GitHub Actions — Secrets
`Settings → Secrets and variables → Actions → Secrets`。`terraform.yml` が参照する。

| Secret 名 | 用途 / 対応する TF 変数 | 必須 | 値の作り方・取得元 |
|---|---|---|---|
| `AWS_ROLE_ARN` | OIDC で AssumeRole する IAM ロール（`role-to-assume`） | ✅ | bootstrap 出力 `ci_role_arn` |
| `JWT_SECRET` | JWT 署名鍵（`TF_VAR_jwt_secret`） | ✅ | `openssl rand -base64 48` |
| `ACCOUNT1_PASSWORD_HASH` | アカウント1の初期パスワード bcrypt ハッシュ | ✅ | `go run ./cmd/hashpw '<パスワード>'` |
| `ACCOUNT2_PASSWORD_HASH` | アカウント2の初期パスワード bcrypt ハッシュ | ✅ | `go run ./cmd/hashpw '<パスワード>'` |
| `ACCOUNT1_LOGINID` | アカウント1の初期ログインID（`TF_VAR_account1_login_id`） | ✅ | 任意に決める |
| `ACCOUNT2_LOGINID` | アカウント2の初期ログインID（`TF_VAR_account2_login_id`） | ✅ | 任意に決める |
| `CLIENT_KEY` | 事前共有クライアントキー（Lambda 検証＋Cloudflare Worker が注入） | ✅※ | `openssl rand -hex 24` |
| `CLOUDFLARE_API_TOKEN` | Cloudflare プロバイダ認証 | ✅※ | Cloudflare ダッシュボードでスコープ限定トークン発行 |
| `CLOUDFLARE_ACCOUNT_ID` | `TF_VAR_cloudflare_account_id` | ✅※ | Cloudflare ダッシュボード右サイド |
| `CLOUDFLARE_ZONE_ID` | ゾーン `tacky0612.net` の Zone ID | ✅※ | Cloudflare ダッシュボード（対象ゾーン） |
| `CLIENT_EMAILS` | Access で許可するメール（`TF_VAR_client_emails`） | ✅※ | JSON 配列文字列 例:`["you@example.com","partner@example.com"]` |
| `BUDGET_ALERT_EMAIL` | コスト超過通知メール | 任意 | メールアドレス。空なら Budgets を作らない |

※印は Cloudflare（`enable_cloudflare = true`）を使う場合に必須。使わない場合は AWS 系（上4つ）のみ必須。

## 2. GitHub Actions — Variables
`Settings → Secrets and variables → Actions → Variables`。非機密の識別子・設定。

| Variable 名 | 用途 | 取得元 |
|---|---|---|
| `TF_STATE_BUCKET` | リモート state の S3 バケット名 | bootstrap 出力 `state_bucket` |

> メンバーの表示名は変数/シークレットで持たず、**アプリ側の既定値（太郎 / 花子）** を使う（`internal/config/config.go`）。変更は画面の表示名編集から行う。

## 3. GitHub Actions — GitHub Pages 用（`deploy-pages.yml`）
GitHub Pages 版フロントをデプロイする場合のみ関係する。

| Secret 名 | 用途 |
|---|---|
| `CLIENT_KEY` | ビルド時に `VITE_CLIENT_KEY` として注入（1 と同じ Secret を共用）。Cloudflare Access 構成（案1）では Worker がヘッダを注入するため不要。公開デモのみを GitHub Pages で配信する場合は未使用でも可 |

## 4. 外部アカウント・前提（手動）
Terraform や CI の前に一度用意するもの。

| 項目 | 用途 | 備考 |
|---|---|---|
| AWS アカウント | Lambda / DynamoDB 等 | bootstrap（`terraform/bootstrap`）で state バケット・OIDC プロバイダ・CIロールを作成 |
| Cloudflare アカウント＋ドメイン | Access / Pages / Worker / DNS | `tacky0612.net` を Cloudflare に追加しネームサーバ委任（DNS は Cloudflare 必須） |
| Cloudflare API トークン | プロバイダ認証（→ `CLOUDFLARE_API_TOKEN`） | Account: Access Apps/Policies・Pages・Workers Scripts 編集 / Zone: DNS・Workers Routes 編集 |
| Cloudflare ↔ GitHub 連携 | Pages のソース連携 | ダッシュボードで一度認可 |

## 5. Lambda 実行時の環境変数（Terraform が設定）
以下は Terraform が上記シークレットから **自動で** Lambda に設定する。手動設定は不要（参考）。

`ACCOUNT1_LOGINID` / `ACCOUNT1_PASSWORD_HASH` / `ACCOUNT2_LOGINID` / `ACCOUNT2_PASSWORD_HASH` / `JWT_SECRET` / `CLIENT_KEY` / `TABLE_NAME` / `ALLOWED_ORIGINS`（表示名は設定せず、アプリ既定の太郎 / 花子を使う）

## 6. ローカル開発
`make up`（Docker Compose）は `docker-compose.yml` にコミット済みの**ローカル専用値**で動く。本番シークレットは不要。

| 変数 | ローカル値 |
|---|---|
| `ACCOUNT1_LOGINID` / `ACCOUNT1_PASSWORD` | `taro` / `taro-password` |
| `ACCOUNT2_LOGINID` / `ACCOUNT2_PASSWORD` | `hanako` / `hanako-password` |
| `JWT_SECRET` | `local-dev-secret` |

> ローカルで本番同等の apply を試す場合のみ、`terraform.tfvars`（gitignore 済み）に機密値を置く。詳細は [deployment.md](./deployment.md)。

## 取り扱い・ローテーション
- いずれもリポジトリにコミットしない（`.gitignore` で `terraform.tfvars` 等を除外済み）。
- 漏洩時は該当値を再生成して GitHub Secrets を更新し、`terraform apply`（＋必要なら再デプロイ）で反映する。
- `JWT_SECRET` を変更すると既存トークンは無効化され、再ログインが必要になる。
