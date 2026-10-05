// Package dynamodb はリポジトリの DynamoDB 実装を提供する（本番・統合テスト用）。
//
// シングルテーブル設計で、全エンティティを1テーブルの PK/SK で表現する。キー設計は
// 下記の定数に集約し、各エンティティの実装は同名のファイル（expense.go / income.go /
// settlement_snapshot.go / recurring_expense.go / direct_transfer.go / reservation.go /
// settings.go / account.go）に分割している。PK 一致のパーティション全件取得は queryByPK を共有する。
package dynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// テーブル全体の PK/SK 設計。詳細は docs/data-model.md を参照。
const (
	expensePKPrefix         = "EXPENSE#"
	monthPKPrefix           = "MONTH#"
	salarySKPrefix          = "SALARY#" // 給与: PK=MONTH#<month> / SK=SALARY#<memberID>
	incomePKPrefix          = "INCOME#" // 追加収入: 単発 INCOME#<month> / 継続 INCOME#RECURRING
	incomeRecurring         = "RECURRING"
	settingsPK              = "SETTINGS"
	weightSK                = "WEIGHT"
	profileSKPrefix         = "PROFILE#"
	closingDaySK            = "CLOSINGDAY"
	recurringPK             = "RECURRING"
	directPKPrefix          = "DIRECTTRANSFER#" // 単発: DIRECTTRANSFER#<month> / 継続: DIRECTTRANSFER#RECURRING
	directRecurring         = "RECURRING"
	snapshotSK              = "SNAPSHOT"         // 精算スナップショット: PK=MONTH#<month> / SK=SNAPSHOT
	reservationPK           = "RESERVATION"      // 予約: PK=RESERVATION / SK=<予約ID>（頻度・対象月は属性）
	reservationFillPKPrefix = "RESERVATIONFILL#" // 予約の入力ロック: PK=RESERVATIONFILL#<month> / SK=<予約ID>
	accountPK               = "ACCOUNT"
	accountSKPrefix         = "ACCT#"
)

// Repositories は DynamoDB 実装のリポジトリ群。
type Repositories struct {
	Expenses     *ExpenseRepository
	Salaries     *SalaryRepository
	Incomes      *IncomeRepository
	Recurring    *RecurringExpenseRepository
	Direct       *DirectTransferRepository
	Reservations *ReservationRepository
	Settings     *SettingsRepository
	Snapshots    *SettlementSnapshotRepository
	Accounts     *AccountRepository
}

// NewRepositories は同一テーブルを共有するリポジトリ群を生成する。
func NewRepositories(client *dynamodb.Client, tableName string) Repositories {
	return Repositories{
		Expenses:     &ExpenseRepository{client: client, table: tableName},
		Salaries:     &SalaryRepository{client: client, table: tableName},
		Incomes:      &IncomeRepository{client: client, table: tableName},
		Recurring:    &RecurringExpenseRepository{client: client, table: tableName},
		Direct:       &DirectTransferRepository{client: client, table: tableName},
		Reservations: &ReservationRepository{client: client, table: tableName},
		Settings:     &SettingsRepository{client: client, table: tableName},
		Snapshots:    &SettlementSnapshotRepository{client: client, table: tableName},
		Accounts:     &AccountRepository{client: client, table: tableName},
	}
}

// queryByPK は PK が pk のアイテムをページングしながらすべて取得し、各アイテムを T として読み取って
// convert でエンティティ E へ変換して返す（該当なしは nil）。各リポジトリの「パーティション全件取得」で共有する。
func queryByPK[T, E any](ctx context.Context, client *dynamodb.Client, table, pk string, convert func(T) (E, error)) ([]E, error) {
	paginator := dynamodb.NewQueryPaginator(client, &dynamodb.QueryInput{
		TableName:              aws.String(table),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
	})
	var list []E
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, raw := range page.Items {
			var item T
			if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
				return nil, err
			}
			e, err := convert(item)
			if err != nil {
				return nil, err
			}
			list = append(list, e)
		}
	}
	return list, nil
}
