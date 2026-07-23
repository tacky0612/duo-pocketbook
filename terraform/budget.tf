# コスト監視（任意）。
#
# budget_alert_email を設定したときのみ、月 $1 を超えたらメール通知する
# AWS Budgets を作成する。無料枠内運用の想定なので、超過は設定ミスや異常アクセスの兆候。
# Budgets 自体は無料（1アカウントあたり最初の2予算まで無料）。

resource "aws_budgets_budget" "cost_alert" {
  count = var.budget_alert_email == "" ? 0 : 1

  name         = "${var.project_name}-cost-alert"
  budget_type  = "COST"
  limit_amount = "1"
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 1
    threshold_type             = "ABSOLUTE_VALUE"
    notification_type          = "ACTUAL"
    subscriber_email_addresses = [var.budget_alert_email]
  }
}
