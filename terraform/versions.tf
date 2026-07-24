terraform {
  # S3 ネイティブロック（use_lockfile）を使うため 1.10 以上
  required_version = ">= 1.10.0"

  # リモート state（暗号化・非公開）。bucket/key/region は init 時に -backend-config で渡す
  # （CI は GitHub Variables、ローカルは backend.hcl）。
  backend "s3" {}

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.52"
    }
  }
}

provider "aws" {
  region = var.region
}

# 認証は環境変数 CLOUDFLARE_API_TOKEN で行う（enable_cloudflare=false のときは未使用）。
provider "cloudflare" {}
