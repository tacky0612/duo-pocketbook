# 非機密の設定（リポジトリにコミットする）。
# 機密値（jwt_secret / member*_password_hash / client_key / cloudflare_* / client_emails 等）は
# コミットしない。CI では GitHub Secrets → 環境変数 TF_VAR_* で注入する（docs/deployment.md 参照）。
# ローカルで apply する場合のみ terraform.tfvars（gitignore 済み）に機密値を置く。

region               = "ap-northeast-1"
reserved_concurrency = 2

# Cloudflare（案1: Access で実行元制限）
enable_cloudflare  = true
app_domain         = "duo-pocketbook.tacky0612.net"
pages_project_name = "duo-pocketbook"
github_owner       = "tacky0612"
github_repo        = "duo-pocketbook"
