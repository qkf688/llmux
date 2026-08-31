package chat

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/repository"
	"github.com/qkf688/llmux/service/credwrite"
)

func closeChatTestDB(t *testing.T) {
	t.Helper()
	if models.DB == nil {
		return
	}
	sqlDB, err := models.DB.DB()
	if err != nil {
		return
	}
	_, _ = sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	_ = sqlDB.Close()
}

// TestApplyCredentialWrites_NonBlockingEnqueue 锁 #6-4-1：请求路径入队不等待慢 DB。
func TestApplyCredentialWrites_NonBlockingEnqueue(t *testing.T) {
	credwrite.ResetDroppedForTest()
	credwrite.EnableTestSyncMode(false)

	block := make(chan struct{})
	var unblock sync.Once
	unblockWorker := func() { unblock.Do(func() { close(block) }) }
	t.Cleanup(func() {
		credwrite.SetHandlers(credwrite.Handlers{
			Cooldown: persistCredentialCooldown,
			AuthFail: persistCredentialAuthFailure,
		})
		unblockWorker()
		credwrite.EnableTestSyncMode(true)
	})

	credwrite.SetHandlers(credwrite.Handlers{
		Cooldown: func(ctx context.Context, credID uint, reason string) error {
			<-block
			return nil
		},
		AuthFail: persistCredentialAuthFailure,
	})

	start := time.Now()
	credwrite.EnqueueCooldown(1, "http_429")
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("EnqueueCooldown blocked for %v, want non-blocking enqueue", elapsed)
	}
	unblockWorker()
}

// TestApplyCredentialCooldown_AsyncEventuallyPersists 异步模式下冷却最终落库。
func TestApplyCredentialCooldown_AsyncEventuallyPersists(t *testing.T) {
	credwrite.ResetDroppedForTest()
	credwrite.EnableTestSyncMode(false)
	credwrite.SetHandlers(credwrite.Handlers{
		Cooldown: persistCredentialCooldown,
		AuthFail: persistCredentialAuthFailure,
	})

	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmux-async-cooldown.db"))
	repository.SetDefault(repository.New(models.DB))
	SetSettingsReader(credCooldownSettingsReader{Cooldown429Sec: 60, CooldownServerSec: 60})
	t.Cleanup(func() {
		SetSettingsReader(nil)
		repository.SetDefault(nil)
		credwrite.EnableTestSyncMode(true)
		closeChatTestDB(t)
	})

	cred := models.Credential{Key: "k", KeyHash: "h"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	before := time.Now()
	applyCredentialCooldown(context.Background(), cred, cooldownReason429)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := credwrite.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.CooldownReason != cooldownReason429 {
		t.Fatalf("CooldownReason = %q, want %q", got.CooldownReason, cooldownReason429)
	}
	if got.CooldownUntil == nil || got.CooldownUntil.Before(before) {
		t.Fatalf("CooldownUntil = %v, want after enqueue time", got.CooldownUntil)
	}
}

func TestApplyCredentialAuthFailure_AsyncEventuallyPersists(t *testing.T) {
	credwrite.ResetDroppedForTest()
	credwrite.EnableTestSyncMode(false)
	credwrite.SetHandlers(credwrite.Handlers{
		Cooldown: persistCredentialCooldown,
		AuthFail: persistCredentialAuthFailure,
	})

	models.Init(context.Background(), filepath.Join(t.TempDir(), "llmux-async-auth.db"))
	repository.SetDefault(repository.New(models.DB))
	SetSettingsReader(credCooldownSettingsReader{AuthFailThreshold: 3})
	t.Cleanup(func() {
		SetSettingsReader(nil)
		repository.SetDefault(nil)
		credwrite.EnableTestSyncMode(true)
		closeChatTestDB(t)
	})

	cred := models.Credential{Key: "k", KeyHash: "h"}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential: %v", err)
	}

	for range 3 {
		applyCredentialAuthFailure(context.Background(), cred)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := credwrite.Flush(ctx); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var got models.Credential
	if err := models.DB.First(&got, cred.ID).Error; err != nil {
		t.Fatalf("reload credential: %v", err)
	}
	if got.Status != models.CredentialStatusTempUnsched {
		t.Fatalf("Status = %q, want temp_unsched", got.Status)
	}
	if got.FailCount != 3 {
		t.Fatalf("FailCount = %d, want 3", got.FailCount)
	}
}

func TestApplyCredentialWrites_WriteStormNonBlocking(t *testing.T) {
	credwrite.ResetDroppedForTest()
	credwrite.EnableTestSyncMode(false)

	block := make(chan struct{})
	var unblock sync.Once
	unblockWorker := func() { unblock.Do(func() { close(block) }) }
	t.Cleanup(func() {
		unblockWorker()
		credwrite.SetHandlers(credwrite.Handlers{
			Cooldown: persistCredentialCooldown,
			AuthFail: persistCredentialAuthFailure,
		})
		credwrite.EnableTestSyncMode(true)
	})

	credwrite.SetHandlers(credwrite.Handlers{
		Cooldown: func(ctx context.Context, credID uint, reason string) error {
			<-block
			return nil
		},
		AuthFail: persistCredentialAuthFailure,
	})

	const n = 200
	start := time.Now()
	for range n {
		credwrite.EnqueueCooldown(1, "http_429")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("enqueue %d cooldowns took %v, want fast non-blocking burst", n, elapsed)
	}

	unblockWorker()
}
