# Lambda 関数 + Function URL
#
# 無料枠の制約:
#   - Lambda 常時無料枠: 100万リクエスト/月 + 40万GB秒/月。128MB/arm64で十分収まる。
#   - Function URL は追加料金なし（API Gatewayは12ヶ月無料のみのため使用しない）。
#   - 認証はアプリケーション側のJWTで行うため authorization_type は NONE。

resource "aws_iam_role" "lambda" {
  name = "${var.project_name}-lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Action    = "sts:AssumeRole"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

# 最小権限: 対象テーブルへのCRUDとログ出力のみ
resource "aws_iam_role_policy" "lambda" {
  name = "${var.project_name}-lambda"
  role = aws_iam_role.lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
        ]
        Resource = aws_dynamodb_table.main.arn
      },
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogStream",
          "logs:PutLogEvents",
        ]
        Resource = "${aws_cloudwatch_log_group.lambda.arn}:*"
      },
    ]
  })
}

resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${var.project_name}"
  retention_in_days = var.log_retention_days
}

resource "aws_lambda_function" "api" {
  function_name = var.project_name
  role          = aws_iam_role.lambda.arn

  filename         = var.lambda_zip_path
  source_code_hash = filebase64sha256(var.lambda_zip_path)

  runtime       = "provided.al2023"
  handler       = "bootstrap"
  architectures = ["arm64"]
  memory_size   = 128
  timeout       = 5

  # 同時実行数の上限を低く固定し、想定外の大量アクセスでも実行時間コストを抑える。
  # -1 のときは未設定（無制限）にする。
  reserved_concurrent_executions = var.reserved_concurrency

  environment {
    variables = {
      # 表示名(ACCOUNT*_NAME)は設定しない。未設定時はアプリ側の既定値（太郎/花子）を使う。
      TABLE_NAME             = aws_dynamodb_table.main.name
      ACCOUNT1_LOGINID       = var.account1_login_id
      ACCOUNT1_PASSWORD_HASH = var.account1_password_hash
      ACCOUNT2_LOGINID       = var.account2_login_id
      ACCOUNT2_PASSWORD_HASH = var.account2_password_hash
      JWT_SECRET             = var.jwt_secret
      ALLOWED_ORIGINS        = join(",", var.allowed_origins)
      CLIENT_KEY             = var.client_key
    }
  }

  depends_on = [aws_cloudwatch_log_group.lambda]
}

resource "aws_lambda_function_url" "api" {
  function_name      = aws_lambda_function.api.function_name
  authorization_type = "NONE"
  # CORSヘッダはアプリケーション側で付与するため、ここでは設定しない
}
