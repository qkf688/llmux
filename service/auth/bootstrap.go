package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// AdminUsername 单管理员的固定用户名。
const AdminUsername = "admin"

// RoleAdmin 是 admin 账号的 Role 字段取值。
const RoleAdmin = "admin"

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
		Role:         RoleAdmin,
	}); err != nil {
		return "", fmt.Errorf("create admin: %w", err)
	}

	return password, nil
}

// LogBootstrap 打印 bootstrap 结果到 stderr（不进结构化日志，避免密码被日志收集器持久化）。
// fromEnv=true 表示密码来自 ADMIN_PASSWORD 环境变量（用户已知，不回显）；
// fromEnv=false 表示随机生成，回显明文密码供运维首次登录。
func LogBootstrap(password string, fromEnv bool) {
	if password == "" {
		return
	}
	if fromEnv {
		fmt.Fprintln(os.Stderr, "admin account created via ADMIN_PASSWORD, please login with that password")
		return
	}
	fmt.Fprintf(os.Stderr, "admin account bootstrapped\n  username: %s\n  password: %s\n  hint: please change password after first login\n", AdminUsername, password)
}
