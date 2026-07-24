variable "project_name" {
  description = "リソース名のプレフィックス"
  type        = string
  default     = "duo-pocketbook"
}

variable "region" {
  description = "AWSリージョン"
  type        = string
  default     = "ap-northeast-1"
}

variable "lambda_zip_path" {
  description = "Lambdaデプロイパッケージ(zip)のパス。`make build-lambda` で生成する"
  type        = string
  default     = "../build/lambda.zip"
}

variable "member1_id" {
  description = "メンバー1のID（ログインIDを兼ねる）"
  type        = string
}

# 表示名はアプリ側の既定値（太郎/花子）を使うため変数化しない。
# 変更は画面（表示名編集）から行う。

variable "member1_password_hash" {
  description = "メンバー1のパスワードのbcryptハッシュ。`go run ./cmd/hashpw '<password>'` で生成する"
  type        = string
  sensitive   = true
}

variable "member2_id" {
  description = "メンバー2のID（ログインIDを兼ねる）"
  type        = string
}

variable "member2_password_hash" {
  description = "メンバー2のパスワードのbcryptハッシュ"
  type        = string
  sensitive   = true
}

variable "jwt_secret" {
  description = "JWT署名用シークレット（十分に長いランダム文字列）"
  type        = string
  sensitive   = true
}

variable "allowed_origins" {
  description = "CORSで許可するオリジン（GitHub PagesのURLなど）"
  type        = list(string)
  default     = ["*"]
}

variable "log_retention_days" {
  description = "CloudWatch Logsの保持日数（無料枠5GB内に収めるため短めにする）"
  type        = number
  default     = 7
}

variable "reserved_concurrency" {
  description = "Lambdaの予約同時実行数（上限）。低く固定して、想定外のトラフィック（bot/DoS）でも実行時間コストが青天井にならないようにする。2人利用なら2で十分。-1で未設定（無制限）。"
  type        = number
  default     = 2
}

variable "client_key" {
  description = "正規クライアント（フロントエンド）識別用の事前共有キー。設定すると X-Client-Key ヘッダが一致しないリクエストを403で弾く。フロントは VITE_CLIENT_KEY として同じ値をビルド時に注入する。空なら無効。公開SPAに埋め込まれるため秘密情報ではなく、botノイズ低減の多層防御。"
  type        = string
  default     = ""
  sensitive   = true
}

variable "budget_alert_email" {
  description = "コスト監視用。設定するとAWS Budgetsで月$1超過時にこのメールへ通知する。空なら作成しない。"
  type        = string
  default     = ""
}

# --- Cloudflare（案1: Access で実行元を制限する構成）。既定は無効 ---

variable "enable_cloudflare" {
  description = "Cloudflare リソース（Access / Worker / Pages / DNS）を Terraform で作成するか。true にする場合は環境変数 CLOUDFLARE_API_TOKEN が必要。"
  type        = bool
  default     = false
}

variable "cloudflare_account_id" {
  description = "Cloudflare アカウントID（enable_cloudflare=true のとき必須）"
  type        = string
  default     = ""
}

variable "cloudflare_zone_id" {
  description = "対象ドメインの Cloudflare ゾーンID（enable_cloudflare=true のとき必須）"
  type        = string
  default     = ""
}

variable "app_domain" {
  description = "アプリを配信するドメイン（例: app.example.com）。Access・Worker ルート・Pages 独自ドメインに使う。"
  type        = string
  default     = ""
}

variable "client_emails" {
  description = "Cloudflare Access で許可するクライアントのメールアドレス（2件）"
  type        = list(string)
  default     = []
}

variable "pages_project_name" {
  description = "Cloudflare Pages プロジェクト名"
  type        = string
  default     = "duo-pocketbook"
}

variable "github_owner" {
  description = "Cloudflare Pages が連携する GitHub オーナー名（Pages のソース連携はダッシュボードで一度認可が必要）"
  type        = string
  default     = ""
}

variable "github_repo" {
  description = "Cloudflare Pages が連携する GitHub リポジトリ名"
  type        = string
  default     = ""
}
