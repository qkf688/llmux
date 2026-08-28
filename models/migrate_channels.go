package models

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"gorm.io/gorm"
)

// migrateLegacyProviders 把存量 Provider 的折叠形态展开为
// 「1 端点 + 1 分组 + 1 内联凭据」，并清空 Config 中的明文 api_key（设计定案第 2 节「凭据加密」）。
//
// 幂等：按 provider 查 endpoints/key_groups 存在性，已有则跳过；
// 整体一个事务，任一步失败回滚——Config 只在凭据搬移成功后才被改写。
// cipher 为 nil 且存在待迁移 Provider 时 panic（禁止半迁移态）。
func migrateLegacyProviders(ctx context.Context, db *gorm.DB, cipher *credentialcrypto.Cipher) {
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var providers []Provider
		if err := tx.Find(&providers).Error; err != nil {
			return fmt.Errorf("list providers: %w", err)
		}
		if len(providers) == 0 {
			return nil // 全新库无需加密（testsupport 等未注入 cipher 的场景不受影响）
		}
		if cipher == nil {
			return errors.New("credentialcrypto cipher not set but providers exist; set CREDENTIAL_ENCRYPTION_KEY before models.Init")
		}
		for i := range providers {
			if err := migrateLegacyProvider(tx, &providers[i], cipher); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
}

// migrateLegacyProvider 展开单个 Provider。
// 分组已存在时直接返回：新表单（S6）写入的内联 key 走 credentials，不会碰 Config.api_key，
// 无需对「已有分组却仍带明文 api_key」的假设场景做防御性清理。
func migrateLegacyProvider(tx *gorm.DB, p *Provider, cipher *credentialcrypto.Cipher) error {
	protocol := string(consts.ProtocolOfType(p.Type))

	// Protocols 空 → 按 type 派生（与前端 protocolOfType 同构，见 consts.ProtocolOfType）。
	if len(p.Protocols) == 0 {
		if err := tx.Model(p).Select("protocols").Updates(&Provider{Protocols: []string{protocol}}).Error; err != nil {
			return fmt.Errorf("fill protocols for provider %d: %w", p.ID, err)
		}
	}

	var endpointCount int64
	if err := tx.Model(&Endpoint{}).Where("provider_id = ?", p.ID).Count(&endpointCount).Error; err != nil {
		return fmt.Errorf("count endpoints for provider %d: %w", p.ID, err)
	}
	if endpointCount == 0 {
		if err := tx.Create(&Endpoint{ProviderID: p.ID, Protocol: protocol, Enabled: true}).Error; err != nil {
			return fmt.Errorf("create endpoint for provider %d: %w", p.ID, err)
		}
	}

	var groupCount int64
	if err := tx.Model(&KeyGroup{}).Where("provider_id = ?", p.ID).Count(&groupCount).Error; err != nil {
		return fmt.Errorf("count key groups for provider %d: %w", p.ID, err)
	}
	if groupCount > 0 {
		return nil
	}

	group := &KeyGroup{ProviderID: p.ID, Name: "默认组", Weight: 1}
	if err := tx.Create(group).Error; err != nil {
		return fmt.Errorf("create key group for provider %d: %w", p.ID, err)
	}

	apiKey := legacyConfigAPIKey(p.Config)
	if apiKey == "" {
		return nil
	}

	encrypted, err := cipher.Encrypt(apiKey)
	if err != nil {
		return fmt.Errorf("encrypt api_key for provider %d: %w", p.ID, err)
	}
	groupID := group.ID
	if err := tx.Create(&Credential{
		Key:     encrypted,
		KeyHash: cipher.Hash(apiKey),
		GroupID: &groupID,
		Status:  CredentialStatusActive,
	}).Error; err != nil {
		return fmt.Errorf("create credential for provider %d: %w", p.ID, err)
	}

	// 凭据搬移成功后才清空明文（事务保证失败回滚时 Config 不动）。
	cleaned, err := sjson.Delete(p.Config, "api_key")
	if err != nil {
		return fmt.Errorf("delete api_key from config for provider %d: %w", p.ID, err)
	}
	if err := tx.Model(p).Update("config", cleaned).Error; err != nil {
		return fmt.Errorf("update config for provider %d: %w", p.ID, err)
	}
	return nil
}

// legacyConfigAPIKey 从 Provider.Config（JSON 字符串）读 api_key。
// Config 非 JSON 时记警告并跳过（该 Provider 本已损坏，不应阻断整库启动迁移）。
func legacyConfigAPIKey(config string) string {
	if config == "" {
		return ""
	}
	if !gjson.Valid(config) {
		slog.Warn("migrate: provider config is not valid JSON, skip api_key migration", "config", config)
		return ""
	}
	return gjson.Get(config, "api_key").String()
}
