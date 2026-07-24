// Package dynamodb はリポジトリの DynamoDB 実装を提供する（本番・統合テスト用）。
//
// シングルテーブル設計で、全エンティティを1テーブルの PK/SK で表現する。キー設計は
// 下記の定数に集約し、各エンティティの実装は同名のファイル（expense.go / income.go /
// settlement_status.go / recurring_expense.go / direct_transfer.go / settings.go /
// account.go）に分割している。
package dynamodb

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// テーブル全体の PK/SK 設計。詳細は docs/data-model.md を参照。
const (
	expensePKPrefix = "EXPENSE#"
	monthPKPrefix   = "MONTH#"
	incomeSKPrefix  = "INCOME#"
	settingsPK      = "SETTINGS"
	weightSK        = "WEIGHT"
	profileSKPrefix = "PROFILE#"
	closingDaySK    = "CLOSINGDAY"
	recurringPK     = "RECURRING"
	directPKPrefix  = "DIRECTTRANSFER#" // 単発: DIRECTTRANSFER#<month> / 継続: DIRECTTRANSFER#RECURRING
	directRecurring = "RECURRING"
	statusSK        = "STATUS"
	accountPK       = "ACCOUNT"
	accountSKPrefix = "ACCT#"
)

// Repositories は DynamoDB 実装のリポジトリ群。
type Repositories struct {
	Expenses  *ExpenseRepository
	Incomes   *IncomeRepository
	Recurring *RecurringExpenseRepository
	Direct    *DirectTransferRepository
	Settings  *SettingsRepository
	Status    *SettlementStatusRepository
	Accounts  *AccountRepository
}

// NewRepositories は同一テーブルを共有するリポジトリ群を生成する。
func NewRepositories(client *dynamodb.Client, tableName string) Repositories {
	return Repositories{
		Expenses:  &ExpenseRepository{client: client, table: tableName},
		Incomes:   &IncomeRepository{client: client, table: tableName},
		Recurring: &RecurringExpenseRepository{client: client, table: tableName},
		Direct:    &DirectTransferRepository{client: client, table: tableName},
		Settings:  &SettingsRepository{client: client, table: tableName},
		Status:    &SettlementStatusRepository{client: client, table: tableName},
		Accounts:  &AccountRepository{client: client, table: tableName},
	}
}
