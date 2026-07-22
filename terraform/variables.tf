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

variable "member1_name" {
  description = "メンバー1の表示名"
  type        = string
}

variable "member1_password_hash" {
  description = "メンバー1のパスワードのbcryptハッシュ。`go run ./cmd/hashpw '<password>'` で生成する"
  type        = string
  sensitive   = true
}

variable "member2_id" {
  description = "メンバー2のID（ログインIDを兼ねる）"
  type        = string
}

variable "member2_name" {
  description = "メンバー2の表示名"
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
