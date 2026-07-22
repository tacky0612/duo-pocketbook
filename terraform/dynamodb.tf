# DynamoDB テーブル（シングルテーブル設計）
#
# 無料枠の制約:
#   - PROVISIONED モード（常時無料枠: 25 RCU / 25 WCU / 25GB）を使う。
#   - PAY_PER_REQUEST(オンデマンド) は常時無料枠の対象外のため使用しない。
#   - 夫婦2人での利用のため 1 RCU / 1 WCU で十分。
resource "aws_dynamodb_table" "main" {
  name         = var.project_name
  billing_mode = "PROVISIONED"

  read_capacity  = 1
  write_capacity = 1

  hash_key  = "PK"
  range_key = "SK"

  attribute {
    name = "PK"
    type = "S"
  }

  attribute {
    name = "SK"
    type = "S"
  }

  point_in_time_recovery {
    # PITRは無料枠対象外のため無効のままにする
    enabled = false
  }
}
