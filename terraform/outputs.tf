output "function_url" {
  description = "APIのエンドポイント（Lambda Function URL）"
  value       = aws_lambda_function_url.api.function_url
}

output "dynamodb_table_name" {
  description = "DynamoDBテーブル名"
  value       = aws_dynamodb_table.main.name
}
