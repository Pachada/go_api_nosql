package dynamo

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// strKey builds a DynamoDB primary key map with a single string attribute.
func strKey(name, value string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		name: &types.AttributeValueMemberS{Value: value},
	}
}

// compositeKey builds a DynamoDB primary key with two string attributes (PK + SK).
func compositeKey(pkName, pkValue, skName, skValue string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		pkName: &types.AttributeValueMemberS{Value: pkValue},
		skName: &types.AttributeValueMemberS{Value: skValue},
	}
}

// buildUpdateExpr converts a map of field->value into a DynamoDB SET expression.
func buildUpdateExpr(updates map[string]interface{}) (expr string, names map[string]string, values map[string]types.AttributeValue, err error) {
	names = make(map[string]string)
	values = make(map[string]types.AttributeValue)
	expr = "SET "
	i := 0
	for k, v := range updates {
		nameKey := fmt.Sprintf("#f%d", i)
		valueKey := fmt.Sprintf(":v%d", i)
		names[nameKey] = k
		av, mErr := attributevalue.Marshal(v)
		if mErr != nil {
			return "", nil, nil, fmt.Errorf("marshal field %s: %w", k, mErr)
		}
		values[valueKey] = av
		if i > 0 {
			expr += ", "
		}
		expr += fmt.Sprintf("%s = %s", nameKey, valueKey)
		i++
	}
	if i == 0 {
		return "", nil, nil, fmt.Errorf("no fields to update")
	}
	return expr, names, values, nil
}
