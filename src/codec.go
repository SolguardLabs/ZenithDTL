package zenithdtl

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func MarshalPretty(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}

func MustJSON(value any) string {
	data, err := MarshalPretty(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func StableAttributes(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make(map[string]string, len(values))
	for _, key := range keys {
		result[key] = values[key]
	}
	return result
}

func EncodeTags(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, ",")
}

func CloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func CloneAmountMap(values map[AssetID]Amount) map[AssetID]Amount {
	result := make(map[AssetID]Amount, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func ParseBoolFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func WithoutFlags(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			continue
		}
		result = append(result, arg)
	}
	return result
}
