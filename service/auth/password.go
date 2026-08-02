// Package auth 提供鉴权相关的纯工具函数：密码哈希与 JWT 签发/解析。
// 不依赖 repository/models/handler，避免循环 import。
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword 返回 bcrypt 哈希后的密码。
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 校验明文密码与哈希是否匹配。
func VerifyPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
