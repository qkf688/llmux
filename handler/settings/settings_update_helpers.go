package settings

import (
	"context"
	"strconv"

	"github.com/atopos31/llmio/models"
	"gorm.io/gorm"
)

func updateBoolSetting(ctx context.Context, key string, value bool) error {
	return updateStringSetting(ctx, key, strconv.FormatBool(value))
}

func updateIntSetting(ctx context.Context, key string, value int) error {
	return updateStringSetting(ctx, key, strconv.Itoa(value))
}

func updateStringSetting(ctx context.Context, key string, value string) error {
	_, err := gorm.G[models.Setting](models.DB).
		Where("key = ?", key).
		Update(ctx, "value", value)
	return err
}
