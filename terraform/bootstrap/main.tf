# ブートストラップ（初回のみローカルで apply）。
#
# CI（GitHub Actions）から Terraform を動かすための土台を作る:
#   - リモート state 用の S3 バケット（暗号化・バージョニング・非公開）
#   - GitHub Actions 用の OIDC プロバイダ
#   - GitHub Actions が AssumeRole する IAM ロール（このリポジトリからのみ・長期キー不要）
#
# この構成自体の state はローカル（bootstrap は一度作れば頻繁に変えないため）。
# 使い方:
#   terraform -chdir=terraform/bootstrap init
#   terraform -chdir=terraform/bootstrap apply \
#     -var 'state_bucket_name=duo-pocketbook-tfstate-xxxxxxxx' \
#     -var 'github_owner=tacky0612' -var 'github_repo=duo-pocketbook'
# 出力の ci_role_arn を GitHub Secrets `AWS_ROLE_ARN`、state_bucket を Variables `TF_STATE_BUCKET` に設定する。

terraform {
  required_version = ">= 1.10.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "region" {
  type    = string
  default = "ap-northeast-1"
}
variable "state_bucket_name" {
  description = "リモート state 用 S3 バケット名（グローバル一意）"
  type        = string
}
variable "github_owner" {
  type = string
}
variable "github_repo" {
  type = string
}
variable "project_name" {
  type    = string
  default = "duo-pocketbook"
}

# --- リモート state バケット ---
resource "aws_s3_bucket" "tfstate" {
  bucket = var.state_bucket_name
}

resource "aws_s3_bucket_versioning" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "tfstate" {
  bucket                  = aws_s3_bucket.tfstate.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# --- GitHub Actions OIDC プロバイダ ---
resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
  thumbprint_list = [
    "6938fd4d98bab03faadb97b34396831e3780aea1",
    "1c58a3a8518e8759bf075b76b750d4f2df264fcd",
  ]
}

# --- CI が AssumeRole する IAM ロール（このリポジトリからのみ） ---
data "aws_iam_policy_document" "assume" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }
    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }
    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      # GitHub の OIDC は sub に不変の数値ID（owner@ID/repo@ID）を含める新形式でも発行される。
      # 旧形式・新形式の両方にマッチさせる（数値IDはワイルドカード。owner@・repo@ で前方一致を固定）。
      values = [
        "repo:${var.github_owner}/${var.github_repo}:*",
        "repo:${var.github_owner}@*/${var.github_repo}@*:*",
      ]
    }
  }
}

resource "aws_iam_role" "ci" {
  name               = "${var.project_name}-ci"
  assume_role_policy = data.aws_iam_policy_document.assume.json
}

data "aws_iam_policy_document" "ci" {
  # リモート state の読み書き
  statement {
    effect    = "Allow"
    actions   = ["s3:ListBucket", "s3:GetBucketVersioning"]
    resources = [aws_s3_bucket.tfstate.arn]
  }
  statement {
    effect    = "Allow"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["${aws_s3_bucket.tfstate.arn}/*"]
  }
  # アプリのリソース管理（利用サービスに限定）
  statement {
    effect = "Allow"
    actions = [
      "lambda:*",
      "dynamodb:*",
      "logs:*",
      "budgets:*",
      "tag:GetResources",
    ]
    resources = ["*"]
  }
  # Lambda 実行ロールの管理（プロジェクトのロール名に限定）
  statement {
    effect = "Allow"
    actions = [
      "iam:CreateRole", "iam:DeleteRole", "iam:GetRole", "iam:PassRole",
      "iam:PutRolePolicy", "iam:DeleteRolePolicy", "iam:GetRolePolicy",
      "iam:ListRolePolicies", "iam:ListAttachedRolePolicies",
      "iam:TagRole", "iam:UntagRole",
    ]
    resources = ["arn:aws:iam::*:role/${var.project_name}-*"]
  }
}

resource "aws_iam_role_policy" "ci" {
  name   = "${var.project_name}-ci"
  role   = aws_iam_role.ci.id
  policy = data.aws_iam_policy_document.ci.json
}

output "ci_role_arn" {
  description = "GitHub Secrets AWS_ROLE_ARN に設定する"
  value       = aws_iam_role.ci.arn
}
output "state_bucket" {
  description = "GitHub Variables TF_STATE_BUCKET に設定する"
  value       = aws_s3_bucket.tfstate.bucket
}
