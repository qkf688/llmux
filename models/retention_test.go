package models

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type retentionProbe struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Name      string
}

func setupRetentionDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 每测独立 DSN，避免 :memory:?cache=shared 串库
	dsn := fmt.Sprintf("file:retention_%s?mode=memory&cache=private", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&retentionProbe{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestEnforceRetentionByOldestID_SoftDelete(t *testing.T) {
	db := setupRetentionDB(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := db.Create(&retentionProbe{Name: "x"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	n, err := EnforceRetentionByOldestID(ctx, db, &retentionProbe{}, 3, RetentionDeleteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("deleted = %d, want 2", n)
	}
	var active int64
	if err := db.Model(&retentionProbe{}).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 3 {
		t.Fatalf("active count = %d, want 3", active)
	}
}

func TestEnforceRetentionByOldestID_UnscopedHardDelete(t *testing.T) {
	db := setupRetentionDB(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := db.Create(&retentionProbe{Name: "x"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// 先软删 1 条，Unscoped count 仍算 5
	if err := db.Where("id = ?", 1).Delete(&retentionProbe{}).Error; err != nil {
		t.Fatal(err)
	}
	n, err := EnforceRetentionByOldestID(ctx, db, &retentionProbe{}, 2, RetentionDeleteOptions{
		UnscopedCount:  true,
		UnscopedDelete: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("deleted = %d, want 3", n)
	}
	var total int64
	if err := db.Unscoped().Model(&retentionProbe{}).Count(&total).Error; err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("unscoped total = %d, want 2", total)
	}
}

func TestEnforceRetentionByOldestID_BeforeDelete(t *testing.T) {
	db := setupRetentionDB(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if err := db.Create(&retentionProbe{Name: "x"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var seen []uint
	_, err := EnforceRetentionByOldestID(ctx, db, &retentionProbe{}, 2, RetentionDeleteOptions{
		UnscopedDelete: true,
		UnscopedCount:  true,
		BeforeDelete: func(_ context.Context, ids []uint) error {
			seen = append(seen, ids...)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("BeforeDelete ids len = %d, want 2", len(seen))
	}
}

func TestEnforceRetentionByOldestID_ZeroRetention(t *testing.T) {
	db := setupRetentionDB(t)
	n, err := EnforceRetentionByOldestID(context.Background(), db, &retentionProbe{}, 0, RetentionDeleteOptions{})
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}
