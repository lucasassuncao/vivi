package vault

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// The Vault API decodes into map[string]any with json.Number for numbers, so
// every field read here has to tolerate more than one concrete type.

func stringField(data map[string]any, key string) string {
	s, _ := data[key].(string)
	return s
}

func numberField(data map[string]any, key string) (int64, error) {
	switch v := data[key].(type) {
	case json.Number:
		return v.Int64()
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("field %q is %T, not a number", key, data[key])
	}
}

func stringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
