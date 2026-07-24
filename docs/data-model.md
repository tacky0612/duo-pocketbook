# データモデル（DynamoDB）

実装: `internal/infrastructure/dynamodb/`

## シングルテーブル設計

テーブルは1つ（デフォルト名 `duo-pocketbook`）。パーティションキー `PK` (S) + ソートキー `SK` (S)。GSIは使用しない。

| エンティティ | PK | SK | 属性 |
|---|---|---|---|
| 共有支出 | `EXPENSE#<yyyy-MM>` | `<expenseID>` | `PaidBy`, `AmountYen`, `Description`, `Date`(YYYY-MM-DD), `CreatedAt`(RFC3339) |
| 月次収入 | `MONTH#<yyyy-MM>` | `INCOME#<memberID>` | `MemberID`, `AmountYen` |
| 精算済みフラグ | `MONTH#<yyyy-MM>` | `STATUS` | `Settled`（bool） |
| 固定費 | `RECURRING` | `<固定費ID>` | `PaidBy`, `AmountYen`, `Description` |
| 精算比重 | `SETTINGS` | `WEIGHT` | `Weights`（memberID→比重のマップ） |
| メンバープロフィール | `SETTINGS` | `PROFILE#<memberID>` | `MemberID`, `Name`, `Color`（上書き設定。未設定項目は保存されない） |
| アカウント | `ACCOUNT` | `ACCT#<accountID>` | `Slot`(1/2), `LoginID`, `PasswordHash`（bcrypt） |

## 設計上のポイント

### AccountID（不変）とログインID（可変）の分離

各アカウントはアカウント作成時に生成される不変の **AccountID**（`acct_<hex>`）で一意に識別される。ログインに用いる **ログインID**（初期値は環境変数 `MEMBER*_ID`）は後から変更でき、AccountID とは独立している。JWT の subject は AccountID であり、支出・収入・比重などのデータはすべて AccountID（＝`memberID`）をキーに保存される。したがってログインIDを変更してもデータの所有関係は変わらない。アカウントは起動時に2スロット分をプロビジョニングし、既存レコードがあればそれを尊重、なければ生成する。

### 支出IDが対象月を内包する

支出IDは `<yyyy-MM>_<ランダム16バイトhex>` 形式（例: `2026-07_75cbcdf9...`）。ドメイン層の `ExpenseID.Month()` でIDから対象月を取り出せるため、`DELETE /expenses/{id}` のようにIDしか渡されない操作でも **GSIやScanなしで** パーティションを特定して `GetItem`/`DeleteItem` できる。

固定費は特定の月に紐づかず単一パーティション（`PK=RECURRING`）に保存される。精算時（`SettlementUsecase`）に `RecurringExpense.AsExpenseFor(month)` で対象月の支出として実体化され、IDは `<yyyy-MM>_recurring-<固定費ID>` 形式になる（DynamoDBには保存されず、精算計算の入力としてのみ生成される）。

### アクセスパターン

| 操作 | DynamoDB操作 |
|---|---|
| 支出の登録/更新 | `PutItem` |
| 支出の月別一覧 | `Query (PK = EXPENSE#<月>)` |
| 支出の取得/削除 | `GetItem` / `DeleteItem`（IDから月を導出してキー構築） |
| 収入の入力（上書き） | `PutItem`（同キーへの上書きが自然に冪等） |
| 収入の月別一覧 | `Query (PK = MONTH#<月> AND begins_with(SK, INCOME#))` |
| 精算済みフラグの取得/更新 | `GetItem` / `PutItem`（`PK=MONTH#<月>, SK=STATUS`） |
| 固定費の登録/更新 | `PutItem`（`PK=RECURRING`） |
| 固定費の取得/削除 | `GetItem` / `DeleteItem` |
| 固定費の一覧 | `Query (PK = RECURRING)` |
| 比重の取得/更新 | `GetItem` / `PutItem`（固定キー） |
| プロフィールの一覧 | `Query (PK = SETTINGS AND begins_with(SK, PROFILE#))` |
| プロフィールの表示名/カラー更新 | `UpdateItem`（単一属性のみ更新し他の属性を維持） |
| アカウントの一覧（起動時・認証時） | `Query (PK = ACCOUNT)` |
| ログインID/パスワードの更新 | `PutItem`（`PK=ACCOUNT, SK=ACCT#<accountID>`） |

### キャパシティ（無料枠の制約）

- **PROVISIONED 1 RCU / 1 WCU**。DynamoDBの常時無料枠は25 RCU/25 WCUであり余裕がある
- **PAY_PER_REQUEST（オンデマンド）は常時無料枠の対象外のため使用しない**（Terraformの `billing_mode` を変更しないこと）
- クライアント2人の操作頻度では1/1で十分。スロットリングが観測された場合でも無料枠内（〜25）での増強に留める

## ローカル環境（DynamoDB Local）

- `docker-compose.yml` で `amazon/dynamodb-local`（インメモリモード）を起動
- アプリは `DYNAMO_ENDPOINT` が設定されている場合のみ、起動時に `EnsureTable` でテーブルを自動作成する（実AWSではTerraform管理のため自動作成しない）
- 認証情報はダミー（`local`/`local`）を使用し、**外部への通信は発生しない**
