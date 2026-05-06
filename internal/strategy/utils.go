package strategy

import (
	"fmt"
	"time"
)

// generateID 生成机会 ID
func generateID(prefix string, parts ...string) string {
	timestamp := time.Now().UnixMilli()
	return fmt.Sprintf("%s_%s_%d", prefix, join(parts), timestamp%100000)
}

// join 连接字符串
func join(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += "_" + p
	}
	return result
}
