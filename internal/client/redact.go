package client

import (
	"encoding/json"
	"strings"
)

const redacted = "[REDACTED]"

// RedactVariables returns a logging-safe copy. GraphQL variable names are
// retained for diagnostics, but every scalar value is removed because Railway
// variables can contain credentials even when the key looks harmless.
func RedactVariables(value any) any {
	return redact(value, false)
}

func redact(value any, preserveScalar bool) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			sensitiveKey := isSensitiveKey(key)
			if sensitiveKey {
				result[key] = redact(child, false)
				continue
			}
			result[key] = redact(child, preserveScalar)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = redact(child, preserveScalar)
		}
		return result
	case json.RawMessage:
		var decoded any
		if json.Unmarshal(typed, &decoded) == nil {
			return redact(decoded, preserveScalar)
		}
		return redacted
	case nil:
		return nil
	default:
		if preserveScalar {
			return typed
		}
		return redacted
	}
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range []string{
		"token", "secret", "password", "authorization", "credential",
		"value", "variables", "access_key", "private_key",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}
