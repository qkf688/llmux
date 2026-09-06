// Package credentialcrypto 提供凭据（API Key）的对称加密、解密与哈希。
//
// 设计定案「凭据加密」：AES-256-GCM，密文存 hex(nonce ‖ ciphertext)；
// KeyHash 用 SHA-256（API key 高熵，无需 keyed HMAC）。
// 本包是叶子包（只依赖标准库 crypto / log），models（迁移）与 repository（CRUD）均可引用。
package credentialcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
)

// Cipher 封装 AES-256-GCM 的加密/解密/哈希原语。
// 线程安全：GCM AEAD 实例的 Seal/Open 可并发调用。
type Cipher struct {
	gcm cipher.AEAD
}

// New 从 32 字节 hex 密钥创建 Cipher。
// 密钥格式与设计定案一致（openssl rand -hex 32），过长/过短/非 hex 一律报错，
// 避免弱密钥静默降级（AES 只支持 128/192/256，固定 32 字节消除歧义）。
func New(hexKey string) (*Cipher, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decode hex key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes hex, got %d bytes", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// Encrypt 加密明文，返回 hex(nonce ‖ ciphertext)。nonce 每次随机 12 字节。
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("read nonce: %w", err)
	}
	sealed := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(sealed), nil
}

// Decrypt 解密 Encrypt 的输出。密钥不匹配或密文被篡改均返回错误（GCM 校验）。
func (c *Cipher) Decrypt(ciphertext string) (string, error) {
	data, err := hex.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("invalid hex ciphertext: %w", err)
	}
	nonceSize := c.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, sealed := data[:nonceSize], data[nonceSize:]
	plain, err := c.gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

// Hash 计算 KeyHash（SHA-256 hex），用于批量导入去重 / 搜索 / 日志关联，
// 无需解密即可完成。空明文返回空串（无有效 key 语义）。
func (c *Cipher) Hash(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// MaskKey 将明文 key 掩码为 sk-****ab12 形态（前缀 3 + **** + 后缀 4），
// 过短时（≤8）一律退化为 ****：1+4 组合在 len 5-8 时几乎完整还原明文。
// 号池与供应商内联 key 共用的脱敏展示原语——明文永不出后端的读路径用它对
// 各条凭据做服务端掩码，避免两套掩码规则漂移。
func MaskKey(plain string) string {
	if len(plain) <= 8 {
		return "****"
	}
	return plain[:3] + "****" + plain[len(plain)-4:]
}

// MaskStored 将「加密存储形态」的 key 转为可展示掩码：cipher 未配置或解密失败
// 一律退化为 ****（掩密文本身形似真实值、毫无意义，绝不能输出），并在失败时
// slog.Warn 留痕。号池列表/详情与供应商内联 key 详情共用同一降级策略——
// 掩码规则的单一来源，改「什么情况出 ****」只需动这里。
// attrs 为附加结构化日志键值（如 "credential_id", id），可空。
func MaskStored(cipher *Cipher, stored string, attrs ...any) string {
	if cipher == nil {
		slog.Warn("credential cipher not configured, masking as ****", attrs...)
		return "****"
	}
	plain, err := cipher.Decrypt(stored)
	if err != nil {
		slog.Warn("credential decrypt failed, masking as ****", append(attrs, "error", err)...)
		return "****"
	}
	return MaskKey(plain)
}
