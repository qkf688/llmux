package credentialcrypto

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestResolveKey_EnvPriority 锁定「环境变量优先」：env 与落盘文件并存时用 env。
func TestResolveKey_EnvPriority(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "credential.key")
	if err := os.WriteFile(keyFile, []byte(strings.Repeat("aa", 32)), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	env := strings.Repeat("bb", 32)
	got, err := ResolveKey(env, keyFile)
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if got != env {
		t.Fatalf("ResolveKey = %q, want env 值", got)
	}
}

// TestResolveKey_ReuseFile 锁定「落盘文件复用」：无 env 时读到已存在的 key 文件，
// 不重新生成。
func TestResolveKey_ReuseFile(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "credential.key")
	persisted := strings.Repeat("cc", 32)
	if err := os.WriteFile(keyFile, []byte(persisted), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	got, err := ResolveKey("", keyFile)
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if got != persisted {
		t.Fatalf("ResolveKey = %q, want 文件内容 %q", got, persisted)
	}
}

// TestResolveKey_GenerateAndPersist 锁定「首启生成 + 落盘」：无 env 无文件时生成
// 32 字节 hex 写盘（0600），同一路径二次解析返回同一密钥。
func TestResolveKey_GenerateAndPersist(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "sub", "credential.key")

	first, err := ResolveKey("", keyFile)
	if err != nil {
		t.Fatalf("ResolveKey: %v", err)
	}
	if len(first) != 64 {
		t.Fatalf("生成密钥长度 = %d, want 64 hex 字符", len(first))
	}

	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("密钥文件未生成: %v", err)
	}
	// Windows 不执行 POSIX 权限位，0600 只在 Unix 上可断言
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("密钥文件权限 = %o, want 600", info.Mode().Perm())
	}

	second, err := ResolveKey("", keyFile)
	if err != nil {
		t.Fatalf("ResolveKey 二次: %v", err)
	}
	if second != first {
		t.Fatal("二次解析应复用同一密钥（重启不丢）")
	}
}
