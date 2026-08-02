package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// AdminUsername 单管理员的固定用户名。
const AdminUsername = "admin"

// BootstrapAdmin 在 users 表为空时创建默认 admin 账号。
// password 为明文密码，内部哈希后存储。返回明文密码（供调用方打印）。
// 若 users 表已有用户则跳过，返回空字符串。
func BootstrapAdmin(ctx context.Context, repo repository.UserRepo, password string) (string, error) {
	count, err := repo.Count(ctx)
	if err != nil {
		return "", fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return "", nil
	}

	hash, err := HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	apiKey, err := GenerateAPIKey()
	if err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}

	if err := repo.Create(ctx, &models.User{
		Username:     AdminUsername,
		PasswordHash: hash,
		APIKey:       apiKey,
		Role:         "admin",
	}); err != nil {
		return "", fmt.Errorf("create admin: %w", err)
	}

	return password, nil
}

// GenerateAPIKey 生成 32 字节随机 hex 字符串作为 API key。
func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "sk-" + hex.EncodeToString(b), nil
}

// GenerateRandomPassword 生成 16 字节随机 hex 字符串作为临时密码。
func GenerateRandomPassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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

// LogBootstrap 打印 bootstrap 结果到日志。
func LogBootstrap(password string) {
	if password == "" {
		return
	}
	slog.Info("admin account bootstrapped",
		"username", AdminUsername,
		"password", password,
		"hint", "please change password after first login",
	)
}
