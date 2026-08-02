package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/qkf688/llmux/repository"
)

// apiKeyPrefix 是 /v1 API key 的统一前缀。
const apiKeyPrefix = "sk-"

// GenerateAPIKey 生成 32 字节随机 hex 字符串作为 API key。
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return apiKeyPrefix + hex.EncodeToString(b), nil
}

// EnsureAPIKey 若用户 API key 为空则生成并写入，返回新 key；否则返回空字符串。
func EnsureAPIKey(ctx context.Context, repo repository.UserRepo, userID uint, currentKey string) (string, error) {
	if currentKey != "" {
		return "", nil
	}
	key, err := GenerateAPIKey()
	if err != nil {
		return "", err
	}
	if err := repo.UpdateAPIKey(ctx, userID, key); err != nil {
		return "", err
	}
	return key, nil
}
