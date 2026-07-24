# Cloudflare（案1: Access で実行元を制限する構成）を Terraform で管理する。
#
# enable_cloudflare = true のときのみ作成する（既定 false。AWS だけの運用を壊さない）。
# 認証は環境変数 CLOUDFLARE_API_TOKEN。DNS は Cloudflare で管理する前提
# （Access / Pages / Workers ルートは Cloudflare エッジ経由が必須のため。Route53 では不可）。
#
# 事前に手動で必要なもの:
#   - ドメインを Cloudflare に追加し、レジストラでネームサーバを Cloudflare に向ける（ゾーン有効化）
#   - Cloudflare API トークン（Account: Access/Pages/Workers Scripts 編集、Zone: DNS/Workers Routes 編集）
#   - Cloudflare の GitHub 連携（Pages のソース連携）をダッシュボードで一度認可しておく

locals {
  cf = var.enable_cloudflare ? 1 : 0
}

# --- /api/* を Lambda Function URL へ転送する Worker ---
# /api を除去して Function URL へプロキシし、秘密ヘッダ X-Client-Key を注入する。
# これにより Lambda の実行元を「Cloudflare 経由（認証済み）」に限定できる。
resource "cloudflare_workers_script" "api_proxy" {
  count      = local.cf
  account_id = var.cloudflare_account_id
  name       = "${var.project_name}-api-proxy"

  content = <<-EOT
    addEventListener("fetch", (event) => {
      event.respondWith(handle(event.request));
    });

    async function handle(request) {
      const url = new URL(request.url);
      const path = url.pathname.replace(/^\/api/, "");
      const origin = FUNCTION_URL.replace(/\/+$/, "");
      const target = origin + (path === "" ? "/" : path) + url.search;
      const headers = new Headers(request.headers);
      headers.delete("host");
      headers.set("X-Client-Key", CLIENT_KEY);
      const init = { method: request.method, headers, redirect: "manual" };
      if (request.method !== "GET" && request.method !== "HEAD") {
        init.body = request.body;
      }
      return fetch(target, init);
    }
  EOT

  plain_text_binding {
    name = "FUNCTION_URL"
    text = aws_lambda_function_url.api.function_url
  }
  secret_text_binding {
    name = "CLIENT_KEY"
    text = var.client_key
  }
}

resource "cloudflare_workers_route" "api" {
  count       = local.cf
  zone_id     = var.cloudflare_zone_id
  pattern     = "${var.app_domain}/api/*"
  script_name = cloudflare_workers_script.api_proxy[0].name
}

# --- Cloudflare Access（未認証をエッジで遮断） ---
resource "cloudflare_zero_trust_access_application" "app" {
  count            = local.cf
  zone_id          = var.cloudflare_zone_id
  name             = var.project_name
  domain           = var.app_domain
  session_duration = "24h"
  # type は既定の self_hosted
}

# メールOTP: IdP リソース不要。許可メールを include で指定する。
resource "cloudflare_zero_trust_access_policy" "allow_clients" {
  count          = local.cf
  application_id = cloudflare_zero_trust_access_application.app[0].id
  zone_id        = var.cloudflare_zone_id
  name           = "allow-clients"
  precedence     = 1
  decision       = "allow"

  include {
    email = var.client_emails
  }
}

# --- Cloudflare Pages（フロント配信・同一ドメイン） ---
# GitHub 連携（source）はダッシュボードでの一度きりの認可が前提。
resource "cloudflare_pages_project" "frontend" {
  count             = local.cf
  account_id        = var.cloudflare_account_id
  name              = var.pages_project_name
  production_branch = "main"

  build_config {
    build_command   = "npm run build"
    destination_dir = "dist"
    root_dir        = "frontend"
  }

  source {
    type = "github"
    config {
      owner             = var.github_owner
      repo_name         = var.github_repo
      production_branch = "main"
    }
  }

  deployment_configs {
    production {
      # 同一オリジン /api 運用（ログイン画面のAPI URL欄は非表示になる）
      environment_variables = {
        VITE_API_BASE = "/api"
      }
    }
    preview {
      environment_variables = {
        VITE_API_BASE = "/api"
      }
    }
  }
}

resource "cloudflare_pages_domain" "frontend" {
  count        = local.cf
  account_id   = var.cloudflare_account_id
  project_name = cloudflare_pages_project.frontend[0].name
  domain       = var.app_domain
}

# アプリのドメインを Pages(<project>.pages.dev) へ向ける（プロキシ）。
resource "cloudflare_record" "app" {
  count   = local.cf
  zone_id = var.cloudflare_zone_id
  name    = var.app_domain
  type    = "CNAME"
  content = cloudflare_pages_project.frontend[0].subdomain
  proxied = true
}
