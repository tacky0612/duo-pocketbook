package dynamodb

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tacky0612/duo-pocketbook/internal/application"
	"github.com/tacky0612/duo-pocketbook/internal/domain"
)

// ReservationRepository は application.ReservationRepository（収入・支出の予約）の DynamoDB 実装。
//
// 予約は頻度に関わらず PK=RESERVATION / SK=<予約ID> の単一パーティションに入る（IDが頻度に依存しないため）。
// 2人分で件数が少ないため、月ごとの一覧はパーティション全体を取得して絞り込む。
// 入力ロックは PK=RESERVATIONFILL#<month> / SK=<予約ID> に、ロックを持つ実データのIDを保持する。
type ReservationRepository struct {
	client *dynamodb.Client
	table  string
}

type reservationItem struct {
	PK                 string `dynamodbav:"PK"`
	SK                 string `dynamodbav:"SK"` // 予約ID
	Kind               string `dynamodbav:"Kind"`
	MemberID           string `dynamodbav:"MemberID"`
	Description        string `dynamodbav:"Description"`
	EstimatedAmountYen int64  `dynamodbav:"EstimatedAmountYen"`
	Month              string `dynamodbav:"Month"`      // YYYY-MM。毎月は空文字
	StartMonth         string `dynamodbav:"StartMonth"` // 毎月の予約の開始月（YYYY-MM）。空文字は制限なし
	// SkippedMonths は「今月はなし」の月（YYYY-MM）の文字列セット。ADD/DELETE で原子的に更新する。
	SkippedMonths []string `dynamodbav:"SkippedMonths,stringset,omitempty"`
}

type fulfillmentItem struct {
	PK       string `dynamodbav:"PK"`
	SK       string `dynamodbav:"SK"`       // 予約ID
	TargetID string `dynamodbav:"TargetID"` // ロックを持つ実データ（支出／収入）のID
}

func reservationKey(id domain.ReservationID) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: reservationPK},
		"SK": &types.AttributeValueMemberS{Value: string(id)},
	}
}

func fulfillmentKey(id domain.ReservationID, month domain.YearMonth) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK": &types.AttributeValueMemberS{Value: reservationFillPKPrefix + month.String()},
		"SK": &types.AttributeValueMemberS{Value: string(id)},
	}
}

func toReservation(item reservationItem) (domain.Reservation, error) {
	month, err := domain.ParseOptionalYearMonth(item.Month)
	if err != nil {
		return domain.Reservation{}, err
	}
	start, err := domain.ParseOptionalYearMonth(item.StartMonth)
	if err != nil {
		return domain.Reservation{}, err
	}
	r, err := domain.NewReservation(item.SK, domain.ReservationKind(item.Kind), domain.MemberID(item.MemberID), item.Description, domain.Money(item.EstimatedAmountYen), month, start)
	if err != nil {
		return domain.Reservation{}, err
	}
	skipped := make([]domain.YearMonth, 0, len(item.SkippedMonths))
	for _, s := range item.SkippedMonths {
		ym, err := domain.ParseYearMonth(s)
		if err != nil {
			return domain.Reservation{}, err
		}
		skipped = append(skipped, ym)
	}
	return r.RestoreSkippedMonths(skipped), nil
}

// Save は予約の内容を UpdateItem で保存（なければ作成）する。
// 「今月はなし」の記録（SkippedMonths）には触れず、同時に行われたスキップを上書きしない。単発の予約は記録を持てないため消す。
func (r *ReservationRepository) Save(ctx context.Context, rsv domain.Reservation) error {
	monthStr, startStr := "", ""
	if rsv.IsRecurring() {
		if !rsv.StartMonth.IsZero() {
			startStr = rsv.StartMonth.String()
		}
	} else {
		monthStr = rsv.Month.String()
	}
	expr := "SET #kind = :kind, #member = :member, #desc = :desc, #est = :est, #month = :month, #start = :start"
	if !rsv.IsRecurring() {
		expr += " REMOVE #skipped"
	}
	names := map[string]string{
		"#kind": "Kind", "#member": "MemberID", "#desc": "Description",
		"#est": "EstimatedAmountYen", "#month": "Month", "#start": "StartMonth",
	}
	if !rsv.IsRecurring() {
		names["#skipped"] = "SkippedMonths"
	}
	values, err := attributevalue.MarshalMap(map[string]any{
		":kind":   string(rsv.Kind),
		":member": string(rsv.MemberID),
		":desc":   rsv.Description,
		":est":    int64(rsv.EstimatedAmount),
		":month":  monthStr,
		":start":  startStr,
	})
	if err != nil {
		return err
	}
	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.table),
		Key:                       reservationKey(rsv.ID),
		UpdateExpression:          aws.String(expr),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return err
}

// AddSkip は「今月はなし」の文字列セットへ月を ADD する（予約が無ければ ErrNotFound）。
func (r *ReservationRepository) AddSkip(ctx context.Context, id domain.ReservationID, month domain.YearMonth) error {
	return r.updateSkip(ctx, id, month, "ADD")
}

// RemoveSkip は「今月はなし」の文字列セットから月を DELETE する（予約が無ければ ErrNotFound）。
func (r *ReservationRepository) RemoveSkip(ctx context.Context, id domain.ReservationID, month domain.YearMonth) error {
	return r.updateSkip(ctx, id, month, "DELETE")
}

func (r *ReservationRepository) updateSkip(ctx context.Context, id domain.ReservationID, month domain.YearMonth, action string) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(r.table),
		Key:                       reservationKey(id),
		UpdateExpression:          aws.String(action + " #skipped :m"),
		ConditionExpression:       aws.String("attribute_exists(PK)"),
		ExpressionAttributeNames:  map[string]string{"#skipped": "SkippedMonths"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":m": &types.AttributeValueMemberSS{Value: []string{month.String()}}},
	})
	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return application.ErrNotFound
	}
	return err
}

// FindByID はIDで予約を取得する。
func (r *ReservationRepository) FindByID(ctx context.Context, id domain.ReservationID) (domain.Reservation, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key:       reservationKey(id),
	})
	if err != nil {
		return domain.Reservation{}, err
	}
	if out.Item == nil {
		return domain.Reservation{}, application.ErrNotFound
	}
	var item reservationItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.Reservation{}, err
	}
	return toReservation(item)
}

// FindAll はすべての予約を返す。
func (r *ReservationRepository) FindAll(ctx context.Context) ([]domain.Reservation, error) {
	return queryByPK(ctx, r.client, r.table, reservationPK, toReservation)
}

// Delete は予約を削除する。
func (r *ReservationRepository) Delete(ctx context.Context, id domain.ReservationID) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.table),
		Key:       reservationKey(id),
	})
	return err
}

// AcquireFulfillment は入力ロックを attribute_not_exists 条件付きの PutItem で取得する。
// 既に存在する場合は強い整合性の GetItem で現在の保持者を返す。
func (r *ReservationRepository) AcquireFulfillment(ctx context.Context, id domain.ReservationID, month domain.YearMonth, target string) (bool, string, error) {
	item, err := attributevalue.MarshalMap(fulfillmentItem{
		PK:       reservationFillPKPrefix + month.String(),
		SK:       string(id),
		TargetID: target,
	})
	if err != nil {
		return false, "", err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.table),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err == nil {
		return true, target, nil
	}
	var ccf *types.ConditionalCheckFailedException
	if !errors.As(err, &ccf) {
		return false, "", err
	}
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(r.table),
		Key:            fulfillmentKey(id, month),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return false, "", err
	}
	var current fulfillmentItem
	if err := attributevalue.UnmarshalMap(out.Item, &current); err != nil {
		return false, "", err
	}
	return false, current.TargetID, nil
}

// ReplaceFulfillment は TargetID が stale のときだけ next に置き換える（条件付き PutItem）。
func (r *ReservationRepository) ReplaceFulfillment(ctx context.Context, id domain.ReservationID, month domain.YearMonth, stale, next string) (bool, error) {
	item, err := attributevalue.MarshalMap(fulfillmentItem{
		PK:       reservationFillPKPrefix + month.String(),
		SK:       string(id),
		TargetID: next,
	})
	if err != nil {
		return false, err
	}
	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                 aws.String(r.table),
		Item:                      item,
		ConditionExpression:       aws.String("TargetID = :stale"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":stale": &types.AttributeValueMemberS{Value: stale}},
	})
	if err == nil {
		return true, nil
	}
	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return false, nil
	}
	return false, err
}
