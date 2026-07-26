package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

// Store 提供基于 Schema 的设置读写服务。
type Store struct {
	db *gorm.DB
}

// NewStore 创建 Store。
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

var (
	defaultMu    sync.RWMutex
	defaultStore *Store
)

// SetDefault 覆盖包级默认 Store（测试可注入独立库）。
func SetDefault(s *Store) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultStore = s
}

// Default 返回包级默认 Store。
// 未显式 SetDefault 时，在 models.DB 已初始化的前提下懒创建 ——
// 包级变量初始化早于 models.Init，此处不能提前捕获 models.DB。
func Default() *Store {
	defaultMu.RLock()
	s := defaultStore
	defaultMu.RUnlock()
	if s != nil {
		return s
	}

	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultStore == nil {
		if models.DB == nil {
			panic("settings: models.DB is nil; call models.Init before settings.Default")
		}
		defaultStore = NewStore(models.DB)
	}
	return defaultStore
}

// GetValue 获取指定 key 的字符串值；若数据库中不存在则返回 schema 默认值序列化后的字符串。
func (s *Store) GetValue(ctx context.Context, key string) (string, error) {
	schema, ok := models.SettingSchemaForKey(key)
	if !ok {
		return "", fmt.Errorf("unknown setting key: %s", key)
	}

	var setting models.Setting
	err := s.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ToString(schema.Default, schema.Type)
		}
		return "", err
	}
	return setting.Value, nil
}

// Get 按 schema 类型获取解析后的值。
func (s *Store) Get(ctx context.Context, key string) (any, error) {
	schema, ok := models.SettingSchemaForKey(key)
	if !ok {
		return nil, fmt.Errorf("unknown setting key: %s", key)
	}
	value, err := s.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}
	return FromString(value, schema.Type)
}

// GetBool 获取 bool 设置；若 key 不存在或解析失败返回 schema 默认值。
func (s *Store) GetBool(ctx context.Context, key string) bool {
	schema, ok := models.SettingSchemaForKey(key)
	if !ok {
		return false
	}
	value, err := s.GetValue(ctx, key)
	if err != nil {
		return schema.Default.(bool)
	}
	parsed, err := FromString(value, models.SettingTypeBool)
	if err != nil {
		return schema.Default.(bool)
	}
	return parsed.(bool)
}

// GetInt 获取 int 设置；若 key 不存在或解析失败返回 schema 默认值。
func (s *Store) GetInt(ctx context.Context, key string) int {
	schema, ok := models.SettingSchemaForKey(key)
	if !ok {
		return 0
	}
	value, err := s.GetValue(ctx, key)
	if err != nil {
		return schema.Default.(int)
	}
	parsed, err := FromString(value, models.SettingTypeInt)
	if err != nil {
		return schema.Default.(int)
	}
	return parsed.(int)
}

// GetString 获取 string 设置；若 key 不存在返回 schema 默认值。
func (s *Store) GetString(ctx context.Context, key string) string {
	schema, ok := models.SettingSchemaForKey(key)
	if !ok {
		return ""
	}
	value, err := s.GetValue(ctx, key)
	if err != nil {
		return schema.Default.(string)
	}
	return value
}

// GetStringSlice 获取 []string 设置；若 key 不存在或解析失败返回 schema 默认值。
func (s *Store) GetStringSlice(ctx context.Context, key string) []string {
	schema, ok := models.SettingSchemaForKey(key)
	if !ok {
		return nil
	}
	value, err := s.GetValue(ctx, key)
	if err != nil {
		return schema.Default.([]string)
	}
	parsed, err := FromString(value, models.SettingTypeStringSlice)
	if err != nil {
		return schema.Default.([]string)
	}
	return parsed.([]string)
}

// GetJSON 将 JSON 设置反序列化到 dest；若 key 不存在使用 schema 默认值。
func (s *Store) GetJSON(ctx context.Context, key string, dest any) error {
	schema, ok := models.SettingSchemaForKey(key)
	if !ok {
		return fmt.Errorf("unknown setting key: %s", key)
	}
	value, err := s.GetValue(ctx, key)
	if err != nil {
		defaultJSON, merr := json.Marshal(schema.Default)
		if merr != nil {
			return merr
		}
		value = string(defaultJSON)
	}
	return json.Unmarshal([]byte(value), dest)
}

// Set 设置 key 的值（字符串形式）。
func (s *Store) Set(ctx context.Context, key, value string) error {
	return s.db.WithContext(ctx).
		Where("key = ?", key).
		Assign(&models.Setting{Key: key, Value: value}).
		FirstOrCreate(&models.Setting{}).Error
}

// SetTyped 根据 schema 类型序列化并设置值。
func (s *Store) SetTyped(ctx context.Context, key string, value any) error {
	schema, ok := models.SettingSchemaForKey(key)
	if !ok {
		return fmt.Errorf("unknown setting key: %s", key)
	}
	str, err := ToString(value, schema.Type)
	if err != nil {
		return err
	}
	return s.Set(ctx, key, str)
}

// List 返回数据库中所有 setting 记录。
func (s *Store) List(ctx context.Context) ([]models.Setting, error) {
	var settings []models.Setting
	if err := s.db.WithContext(ctx).Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}
