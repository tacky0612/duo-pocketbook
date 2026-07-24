package dynamodb

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// SalaryRepository は application.SalaryRepository の DynamoDB 実装。
// PK=MONTH#<month>、SK=SALARY#<memberID> でメンバーごと・月ごとに1件を保持する。
type SalaryRepository struct {
	client *dynamodb.Client
	table  string
}

type salaryItem struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	MemberID  string `dynamodbav:"MemberID"`
	AmountYen int64  `dynamodbav:"AmountYen"`
}

// Save は給与を保存（上書き）する。
func (r *SalaryRepository) Save(ctx context.Context, salary domain.Salary) error {
	item, err := attributevalue.MarshalMap(salaryItem{
		PK:        monthPKPrefix + salary.Month.String(),
		SK:        salarySKPrefix + string(salary.MemberID),
		MemberID:  string(salary.MemberID),
		AmountYen: int64(salary.Amount),
	})
	if err != nil {
		return err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.table),
		Item:      item,
	})
	return err
}

// FindByMonth は対象月の給与を返す。
func (r *SalaryRepository) FindByMonth(ctx context.Context, month domain.YearMonth) ([]domain.Salary, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: monthPKPrefix + month.String()},
			":sk": &types.AttributeValueMemberS{Value: salarySKPrefix},
		},
	})
	if err != nil {
		return nil, err
	}
	var salaries []domain.Salary
	for _, raw := range out.Items {
		var item salaryItem
		if err := attributevalue.UnmarshalMap(raw, &item); err != nil {
			return nil, err
		}
		salary, err := domain.NewSalary(month, domain.MemberID(item.MemberID), domain.Money(item.AmountYen))
		if err != nil {
			return nil, err
		}
		salaries = append(salaries, salary)
	}
	return salaries, nil
}
