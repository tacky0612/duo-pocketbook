// Package dynamodb はリポジトリの DynamoDB 実装を提供する。
// シングルテーブル設計:
//   - 支出:   PK=EXPENSE#<yyyy-MM>  SK=<expenseID>
//   - 収入:   PK=MONTH#<yyyy-MM>    SK=INCOME#<memberID>
//   - 比重:   PK=SETTINGS           SK=WEIGHT
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// NewClient は DynamoDB クライアントを生成する。
// endpoint が指定された場合（DynamoDB Local）はそのエンドポイントとダミー認証情報を使う。
func NewClient(ctx context.Context, endpoint string) (*dynamodb.Client, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if endpoint != "" {
		opts = append(opts,
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
			awsconfig.WithRegion("ap-northeast-1"),
		)
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("AWS設定の読み込みに失敗しました: %w", err)
	}
	return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	}), nil
}

// EnsureTable はテーブルが存在しない場合に作成する（DynamoDB Local 用）。
// DynamoDB Local の起動を待つため、一定時間リトライする。
func EnsureTable(ctx context.Context, client *dynamodb.Client, tableName string) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		})
		if err == nil {
			return nil
		}

		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			if createErr := createTable(ctx, client, tableName); createErr == nil {
				return waitTableActive(ctx, client, tableName)
			}
			// 同時作成などで失敗した場合はリトライへ
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("テーブルの確認に失敗しました: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func createTable(ctx context.Context, client *dynamodb.Client, tableName string) error {
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String(tableName),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("PK"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("SK"), KeyType: types.KeyTypeRange},
		},
		BillingMode: types.BillingModeProvisioned,
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
	})
	return err
}

func waitTableActive(ctx context.Context, client *dynamodb.Client, tableName string) error {
	waiter := dynamodb.NewTableExistsWaiter(client)
	return waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	}, 60*time.Second)
}
