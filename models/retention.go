package models

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// RetentionDeleteOptions 控制「超限删最旧」的 count/delete 语义差异。
type RetentionDeleteOptions struct {
	// UnscopedCount 为 true 时 Count 使用 Unscoped（含软删堆积，ChatLog 需要）。
	UnscopedCount bool
	// UnscopedDelete 为 true 时主表 Delete 使用 Unscoped（硬删）；false 为普通 Delete（软删）。
	UnscopedDelete bool
	// BeforeDelete 在删除主表前调用（如先删 ChatIO）。
	BeforeDelete func(ctx context.Context, ids []uint) error
}

// EnforceRetentionByOldestID 通用「count → 超限 → 取最旧 N 条 id → 删除」骨架。
// model 为 GORM 模型零值指针，如 &ChatLog{}。
// retention<=0 时不清理，返回 deleted=0。
func EnforceRetentionByOldestID(
	ctx context.Context,
	db *gorm.DB,
	model any,
	retention int,
	opt RetentionDeleteOptions,
) (deleted int, err error) {
	if retention <= 0 {
		return 0, nil
	}
	if db == nil {
		return 0, fmt.Errorf("retention: db is nil")
	}

	countDB := db.WithContext(ctx).Model(model)
	if opt.UnscopedCount {
		countDB = countDB.Unscoped()
	}
	var total int64
	if err := countDB.Count(&total).Error; err != nil {
		return 0, err
	}
	if int(total) <= retention {
		return 0, nil
	}

	deleteCount := int(total) - retention
	findDB := db.WithContext(ctx).Model(model)
	if opt.UnscopedCount {
		// 与 ChatLog 一致：统计含软删时，取最旧也要 Unscoped
		findDB = findDB.Unscoped()
	}
	var ids []uint
	if err := findDB.Order("id ASC").Limit(deleteCount).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if opt.BeforeDelete != nil {
		if err := opt.BeforeDelete(ctx, ids); err != nil {
			return 0, err
		}
	}

	delDB := db.WithContext(ctx)
	if opt.UnscopedDelete {
		delDB = delDB.Unscoped()
	}
	if err := delDB.Where("id IN ?", ids).Delete(model).Error; err != nil {
		return 0, err
	}
	return len(ids), nil
}
