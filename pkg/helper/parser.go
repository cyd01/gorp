package helper

import (
	"fmt"
	"time"
)

func ParseStringOrDefault(config map[string]interface{}, key, defaultValue string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return defaultValue
}

func ParseInt(config map[string]interface{}, key string, defaultValue int) int {
	if value, ok := config[key].(int); ok {
		return value
	}
	return defaultValue
}

func ParseStringMap(config map[string]interface{}, key string) (map[string]string, error) {
	result := make(map[string]string)
	if raw, ok := config[key].(map[string]interface{}); ok {
		for k, v := range raw {
			if str, ok := v.(string); ok {
				result[k] = str
			} else {
				return nil, fmt.Errorf("invalid value for %s.%s: expected string", key, k)
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("missing or invalid %s", key)
}

func ParseStringSlice(config map[string]interface{}, key string) ([]string, error) {
	if raw, ok := config[key].([]interface{}); ok {
		result := make([]string, 0, len(raw))
		for _, item := range raw {
			if str, ok := item.(string); ok {
				result = append(result, str)
			} else {
				return nil, fmt.Errorf("invalid value for %s: expected string", key)
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("missing or invalid %s", key)
}

func ParseDuration(config map[string]interface{}, key string, defaultValue time.Duration) (time.Duration, error) {
	if raw, ok := config[key].(string); ok {
		return time.ParseDuration(raw)
	}
	return defaultValue, nil
}
