package credentialcrypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ResolveKey 解析凭据加密密钥，优先级：环境变量 > 落盘密钥文件 > 生成并落盘。
//
// envValue 非空直接返回；否则读 keyFile（存在则复用）；两者都没有时生成 32 字节
// hex 密钥并写入 keyFile（目录自动创建，文件权限 0600），同时向 stderr 告警。
// 与 ADMIN_PASSWORD 不同，生成的密钥必须落盘而非仅打印：密码丢了可重设，
// 加密密钥丢了旧密文全部不可读。
func ResolveKey(envValue, keyFile string) (string, error) {
	if envValue != "" {
		return envValue, nil
	}
	if data, err := os.ReadFile(keyFile); err == nil {
		return string(data), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read credential key file: %w", err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate credential key: %w", err)
	}
	hexKey := hex.EncodeToString(key)

	if err := os.MkdirAll(filepath.Dir(keyFile), 0o755); err != nil {
		return "", fmt.Errorf("mkdir credential key dir: %w", err)
	}
	if err := os.WriteFile(keyFile, []byte(hexKey), 0o600); err != nil {
		return "", fmt.Errorf("write credential key file: %w", err)
	}
	fmt.Fprintf(os.Stderr,
		"CREDENTIAL_ENCRYPTION_KEY not set: generated and persisted to %s (0600).\n"+
			"  Set the env var to manage the key explicitly; losing this key makes all stored credentials unreadable.\n",
		keyFile)
	return hexKey, nil
}

var (
	defaultMu sync.RWMutex
	defaultC  *Cipher
)

// SetDefault 设置包级默认 Cipher（通常在启动装配时设置一次，早于 models.Init——
// 存量迁移需要它加密 api_key）。
func SetDefault(c *Cipher) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultC = c
}

// Default 返回包级默认 Cipher；未设置时返回 nil（调用方需处理）。
func Default() *Cipher {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultC
}
