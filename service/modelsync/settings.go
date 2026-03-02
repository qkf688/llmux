package modelsync

import (
	"context"
	"strconv"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func (s *Service) getSettingBool(ctx context.Context, key string) (bool, error) {
	setting, err := gorm.G[models.Setting](s.db).Where("key = ?", key).First(ctx)
	if err != nil {
		return false, err
	}
	return setting.Value == "true", nil
}

func (s *Service) getSettingInt(ctx context.Context, key string) (int, error) {
	setting, err := gorm.G[models.Setting](s.db).Where("key = ?", key).First(ctx)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(setting.Value)
}
