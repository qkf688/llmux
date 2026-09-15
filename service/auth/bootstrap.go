package auth

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/qkf688/llmux/common"
	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
)

// AdminUsername 单管理员的固定用户名。
const AdminUsername = "admin"

// RoleAdmin 是 admin 账号的 Role 字段取值。
const RoleAdmin = "admin"

// DefaultAPIKeyName 首启自动创建的首把调用 key 的名称（多 api-key 方案 Ruling：
// users.api_key 退役后新部署需要首把 key 来源，沿 ADMIN_PASSWORD 随机生成先例）。
const DefaultAPIKeyName = "default"

// BootstrapResult 首启装配的产物；零值表示已有用户、整体跳过。
// APIKey 为 default 调用 key 的明文，仅此返回值可见（库里只有密文），由
// LogBootstrap 打印一次供运维配置客户端。
type BootstrapResult struct {
	Password string
	APIKey   string
}

// BootstrapAdmin 在 users 表为空时创建默认 admin 账号 + 首把 default 调用 key。
// password 为明文密码，内部哈希后存储。
// admin 与 default key 同事务进退：半途失败不得留下「有 admin 无可用 key」的
// 半套状态（重启守卫 count>0 会跳过补建，key 将永久缺失）。
// 若 users 表已有用户则跳过，返回零值。
func BootstrapAdmin(ctx context.Context, repos *repository.Repositories, password string) (BootstrapResult, error) {
	count, err := repos.User.Count(ctx)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return BootstrapResult{}, nil
	}

	cipher := credentialcrypto.Default()
	if cipher == nil {
		return BootstrapResult{}, errors.New("credential encryption key not configured; refusing bootstrap without cipher")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("hash password: %w", err)
	}
	defaultKey, err := common.GenerateCallerKeyValue()
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("generate default api key: %w", err)
	}
	encrypted, err := cipher.Encrypt(defaultKey)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("encrypt default api key: %w", err)
	}

	err = repos.RunInTx(ctx, func(tx *repository.Repositories) error {
		user := &models.User{
			Username:     AdminUsername,
			PasswordHash: hash,
			Role:         RoleAdmin,
		}
		if err := tx.User.Create(ctx, user); err != nil {
			return err
		}
		return tx.ApiKey.Create(ctx, &models.ApiKey{
			OwnerID: user.ID,
			Name:    DefaultAPIKeyName,
			Key:     encrypted,
			KeyHash: cipher.Hash(defaultKey),
			Enabled: true,
		})
	})
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("bootstrap admin: %w", err)
	}

	return BootstrapResult{Password: password, APIKey: defaultKey}, nil
}

// LogBootstrap 打印 bootstrap 结果到 stderr（不进 slog 结构化管道，避免密码被日志
// 收集器按结构化字段持久化；注意容器化部署下 stderr 本身仍会进 `docker logs`，
// 与 ADMIN_PASSWORD 先例同等暴露面）。
// fromEnv=true 表示密码来自 ADMIN_PASSWORD 环境变量（用户已知，不回显）；
// 随机生成的密码与 default key 明文各打印一次。零值（已有用户跳过）不打印。
func LogBootstrap(res BootstrapResult, fromEnv bool) {
	if res.Password == "" && res.APIKey == "" {
		return
	}
	if fromEnv {
		fmt.Fprintln(os.Stderr, "admin account created via ADMIN_PASSWORD, please login with that password")
	} else {
		fmt.Fprintf(os.Stderr, "admin account bootstrapped\n  username: %s\n  password: %s\n  hint: please change password after first login\n", AdminUsername, res.Password)
	}
	if res.APIKey != "" {
		fmt.Fprintf(os.Stderr, "default /v1 api key (shown once, store it now): %s\n", res.APIKey)
	}
}
