# API 利用ガイド

正式な契約は OpenAPI 定義 [`api/openapi.yaml`](../api/openapi.yaml)（OpenAPI 3.1）を参照。ここでは利用の流れを説明する。

> `api/openapi.yaml` は **Go コードが正（source of truth）**。`internal/web` のハンドラに付けた [swag](https://github.com/swaggo/swag) 注釈と DTO 型から `make openapi` で自動生成する（**手で編集しない**）。API を変更したらハンドラの注釈・DTO を直して `make openapi` を実行する。CI の `openapi-check` ジョブが未再生成を検出する。

## ベースURL

- ローカル: `http://localhost:8080`（`make up` で起動）
- 本番: Lambda Function URL（`terraform output function_url` で確認）

## 認証

2アカウントのみの固定メンバー制。`POST /login` で JWT を取得し、以降のリクエストの `Authorization: Bearer <token>` に付与する。

```bash
TOKEN=$(curl -s -X POST $BASE/login \
  -d '{"memberId":"taro","password":"taro-password"}' | jq -r .token)
```

- トークン有効期限はデフォルト30日（`TOKEN_TTL_HOURS` で変更可）
- `/health` と `/login` 以外はすべて認証必須
- ログインの `memberId` フィールドには**ログインID**（可変のユーザー名）を渡す。初期値は環境変数 `ACCOUNT1_LOGINID`/`ACCOUNT2_LOGINID`
- JWT の subject は **AccountID**（`acct_...`）。アカウント作成時から不変で、ログインIDを変更しても変わらない。各種データは AccountID をキーに保存される

## CORS

`ALLOWED_ORIGINS`（カンマ区切り、デフォルト `*`）に一致するオリジンからのリクエストのみ許可する。`OPTIONS` プリフライトには `204` を返す。

## エンドポイント一覧

| メソッド/パス | 内容 |
|---|---|
| `GET /health` | ヘルスチェック（認証不要） |
| `POST /login` | ログイン、JWT発行（認証不要） |
| `GET /members` | メンバー一覧（2人） |
| `PUT /members/{id}` | メンバーの表示名・カラーの更新（指定した項目のみ上書き） |
| `GET /account` | 認証中アカウントの情報（AccountID・ログインID・表示名） |
| `PUT /account/login-id` | ログインIDの変更（英数字と `. _ -`・32文字以内・重複不可） |
| `PUT /account/password` | パスワードの変更（現在のパスワード必須・新パスワードは8文字以上） |
| `POST /expenses` | 共有支出の登録 |
| `GET /expenses?month=YYYY-MM` | 共有支出の月別一覧（日付降順） |
| `PUT /expenses/{id}` | 共有支出の更新 |
| `DELETE /expenses/{id}` | 共有支出の削除（どちらのメンバーでも可） |
| `PUT /months/{month}/salaries/{memberId}` | 月次給与の入力（上書き。メンバーごと1件） |
| `GET /months/{month}/salaries` | 月次給与の一覧 |
| `POST /incomes` | 追加収入の登録（`month` 空で毎月継続・指定でその月のみの単発） |
| `GET /incomes?month=YYYY-MM` | 指定月に適用される追加収入の一覧（継続分＋当月単発分） |
| `PUT /incomes/{id}` | 追加収入の更新（メンバー・金額・内容。継続/単発と対象月は不変） |
| `DELETE /incomes/{id}` | 追加収入の削除 |
| `GET /months/{month}/settlement` | 月次精算の取得 |
| `PUT /months/{month}/settlement/status` | 精算済みフラグの更新 |
| `GET /settlements/history?from=YYYY-MM&to=YYYY-MM` | 精算履歴の取得（新しい月順） |
| `POST /recurring-expenses` | 固定費の登録 |
| `GET /recurring-expenses` | 固定費の一覧 |
| `PUT /recurring-expenses/{id}` | 固定費の更新 |
| `DELETE /recurring-expenses/{id}` | 固定費の削除 |
| `POST /direct-transfers` | 立替精算の登録（`month` 空で毎月継続・指定でその月のみの単発） |
| `GET /direct-transfers?month=YYYY-MM` | 指定月に適用される立替精算の一覧（継続分＋当月単発分） |
| `PUT /direct-transfers/{id}` | 立替精算の更新（送金元・金額・内容。継続/単発と対象月は不変） |
| `DELETE /direct-transfers/{id}` | 立替精算の削除 |
| `GET /settings/weight` | 精算比重の取得（未設定時 1:1） |
| `PUT /settings/weight` | 精算比重の更新 |
| `GET /settings/closing-day` | 締め日の取得（未設定時 1＝暦月どおり） |
| `PUT /settings/closing-day` | 締め日の更新（1〜31） |

> **締め日と対象月**: 締め日（`/settings/closing-day`）を 2 以上に設定すると、`month=YYYY-MM` を扱うエンドポイント（支出一覧・精算・履歴）はその月を**締め期間**として集計する（例: 締め日15 なら 7月 = 6/15〜7/14）。支出自体は支出日の暦月をキーに保存され、締め日を変えても保存先は変わらない。詳細は [settlement.md](settlement.md#締め日)。

## 典型フロー

```bash
BASE=http://localhost:8080

# 1. 支出を登録（それぞれの立場で）
curl -X POST $BASE/expenses -H "Authorization: Bearer $TOKEN" \
  -d '{"paidBy":"taro","amountYen":20000,"description":"家賃","date":"2026-07-01"}'

# 2. 月次給与を入力（精算の可否判定に使う基本の収入）
curl -X PUT $BASE/months/2026-07/salaries/taro -H "Authorization: Bearer $TOKEN" \
  -d '{"amountYen":100000}'
curl -X PUT $BASE/months/2026-07/salaries/hanako -H "Authorization: Bearer $TOKEN2" \
  -d '{"amountYen":50000}'

# 2b. （任意）給与とは別の追加収入を登録（副業など・日付なし）
curl -X POST $BASE/incomes -H "Authorization: Bearer $TOKEN" \
  -d '{"memberId":"taro","amountYen":20000,"description":"副業","month":""}'

# 3. 精算を取得
curl $BASE/months/2026-07/settlement -H "Authorization: Bearer $TOKEN"
```

精算レスポンスの例（立替精算 太郎→花子 3,000円 を含む月）:

```json
{
  "month": "2026-07",
  "totalExpenseYen": 40000,
  "settled": false,
  "members": [
    {"id": "taro",   "name": "太郎", "weight": 1, "incomeYen": 100000, "paidExpenseYen": 20000, "disposableYen": 55000},
    {"id": "hanako", "name": "花子", "weight": 1, "incomeYen": 50000,  "paidExpenseYen": 20000, "disposableYen": 55000}
  ],
  "transfer":           {"from": "taro", "to": "hanako", "amountYen": 28000},
  "settlementTransfer": {"from": "taro", "to": "hanako", "amountYen": 25000},
  "directTransfer":     {"from": "taro", "to": "hanako", "amountYen": 3000},
  "totalDirectTransferYen": 3000
}
```

- `transfer` は実際に振り込む金額で、**比重按分の精算分 `settlementTransfer` と立替精算分 `directTransfer` を合算**したもの。いずれも `null` の場合は不要。
- `settlementTransfer` / `directTransfer` は内訳（方向が逆になることもある）。`disposableYen`（精算後の可処分所得）は**共有支出の比重按分のみを反映**し、立替精算は含めない。
- `totalDirectTransferYen` は当月に適用された立替精算の総額（方向を問わない絶対額の合計）。
- `settled` は `PUT /months/{month}/settlement/status` で更新する精算済みフラグ（振込を実施したかどうかの記録用で、精算計算そのものには影響しない）。

固定費（`recurring-expenses`）が登録されている場合、精算計算時に対象月の共有支出として自動的に合算される。立替精算（`direct-transfers`）は共有支出とは別枠で、比重按分せずに振込額へそのまま加算される（詳細は [settlement.md](settlement.md#立替精算共有支出とは別枠の送金)）。追加収入（`incomes`）は各メンバーの給与と合算して収入（`incomeYen`）に反映される（詳細は [settlement.md](settlement.md#収入給与と追加収入)）。

## エラーレスポンス

すべてのエラーは共通形式:

```json
{"error": {"code": "INCOME_NOT_READY", "message": "..."}}
```

| HTTP | code | 意味 |
|---|---|---|
| 400 | `VALIDATION_ERROR` | 入力値がドメイン制約を満たさない（金額・年月形式・不明メンバー等） |
| 401 | `UNAUTHORIZED` | 未認証・トークン無効・認証情報誤り |
| 403 | `FORBIDDEN` | 事前共有クライアントキー（`X-Client-Key`）が不一致（`CLIENT_KEY` 設定時のみ） |
| 404 | `NOT_FOUND` | 対象データが存在しない |
| 409 | `INCOME_NOT_READY` | 精算に必要な両メンバーの給与が未入力 |
| 429 | `RATE_LIMITED` | リクエストが多すぎる（`/login` のIP単位レート制限） |
| 500 | `INTERNAL` | 内部エラー |

### アクセス制限ヘッダ

`CLIENT_KEY` を設定して運用する場合、`/health` と CORS プリフライト（OPTIONS）を除く全リクエストに `X-Client-Key: <キー>` を付与する必要がある（不一致は 403 `FORBIDDEN`）。フロントエンドはビルド時の `VITE_CLIENT_KEY` から自動付与する。詳細は [デプロイガイド](./deployment.md) の「アクセス制限とコスト最適化」を参照。

## ドキュメントページ（自動生成・ホスティング）

`api/openapi.yaml` から **ReDoc** 形式の API ドキュメント HTML を生成し、フロントと同じ配信物に含めている。生成はフロントのビルド（`npm run build` → `npm run docs:api`、`@redocly/cli` の `build-docs`）に組み込まれており、GitHub Pages・Cloudflare Pages の両方へ自動デプロイされる。

- 公開パス: **`/api-docs.html`**（例: `https://<username>.github.io/duo-pocketbook/api-docs.html`、Cloudflare 構成では `https://<app_domain>/api-docs.html`）
- ローカル生成: `cd frontend && npm run docs:api`（`dist/api-docs.html` を出力）
- 単体プレビュー: `npx @redocly/cli build-docs api/openapi.yaml -o /tmp/api-docs.html`

## 外部ツール連携

`api/openapi.yaml` を Swagger UI / Postman / openapi-generator 等に読み込めばクライアントを自動生成できる。この定義は Go コードから `make openapi` で生成されるため、API を変更したら注釈・DTO を直して再生成すること（[api.md 冒頭](#api-利用ガイド)参照）。
