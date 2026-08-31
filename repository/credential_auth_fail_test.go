package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/qkf688/llmux/models"
	"gorm.io/gorm"
)

func newCredentialTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Credential{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestIncrementFailCountAndStopIfThreshold(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newCredentialTestDB(t))

	t.Run("below threshold only increments", func(t *testing.T) {
		cred := models.Credential{Key: "k", KeyHash: "h", Status: models.CredentialStatusActive}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		for i := 1; i <= 2; i++ {
			n, err := repo.IncrementFailCountAndStopIfThreshold(ctx, cred.ID, 3, "auth_fail")
			if err != nil {
				t.Fatalf("increment step %d: %v", i, err)
			}
			if n != 1 {
				t.Fatalf("rows step %d = %d, want 1", i, n)
			}
			got, err := repo.Get(ctx, cred.ID)
			if err != nil {
				t.Fatalf("get step %d: %v", i, err)
			}
			if got.FailCount != i {
				t.Fatalf("FailCount step %d = %d, want %d", i, got.FailCount, i)
			}
			if got.Status != models.CredentialStatusActive {
				t.Fatalf("Status step %d = %q, want active", i, got.Status)
			}
		}
	})

	t.Run("at threshold stops active credential", func(t *testing.T) {
		cred := models.Credential{Key: "k2", KeyHash: "h2", FailCount: 2, Status: models.CredentialStatusActive}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		n, err := repo.IncrementFailCountAndStopIfThreshold(ctx, cred.ID, 3, "auth_fail")
		if err != nil {
			t.Fatalf("increment: %v", err)
		}
		if n != 1 {
			t.Fatalf("rows = %d, want 1", n)
		}
		got, err := repo.Get(ctx, cred.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.FailCount != 3 {
			t.Fatalf("FailCount = %d, want 3", got.FailCount)
		}
		if got.Status != models.CredentialStatusTempUnsched {
			t.Fatalf("Status = %q, want temp_unsched", got.Status)
		}
		if got.CooldownReason != "auth_fail" {
			t.Fatalf("CooldownReason = %q, want auth_fail", got.CooldownReason)
		}
		if got.CooldownUntil != nil {
			t.Fatalf("CooldownUntil != nil, want nil")
		}
	})

	t.Run("does not stop disabled or error status", func(t *testing.T) {
		for _, status := range []string{models.CredentialStatusDisabled, models.CredentialStatusError} {
			t.Run(status, func(t *testing.T) {
				cred := models.Credential{Key: status, KeyHash: status, FailCount: 2, Status: status}
				if err := repo.Create(ctx, &cred); err != nil {
					t.Fatalf("create: %v", err)
				}
				n, err := repo.IncrementFailCountAndStopIfThreshold(ctx, cred.ID, 3, "auth_fail")
				if err != nil {
					t.Fatalf("increment: %v", err)
				}
				if n != 1 {
					t.Fatalf("rows = %d, want 1", n)
				}
				got, err := repo.Get(ctx, cred.ID)
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				if got.Status != status {
					t.Fatalf("Status = %q, want %q", got.Status, status)
				}
				if got.FailCount != 3 {
					t.Fatalf("FailCount = %d, want 3", got.FailCount)
				}
			})
		}
	})

	t.Run("clears residual cooldown on stop", func(t *testing.T) {
		residual := models.Credential{
			Key:            "k3",
			KeyHash:        "h3",
			FailCount:      2,
			Status:         models.CredentialStatusActive,
			CooldownReason: "http_429",
		}
		if err := repo.Create(ctx, &residual); err != nil {
			t.Fatalf("create: %v", err)
		}
		until := time.Now().Add(5 * time.Minute)
		if _, err := repo.UpdateFields(ctx, residual.ID, map[string]any{
			"cooldown_until": until,
		}); err != nil {
			t.Fatalf("set cooldown: %v", err)
		}
		if _, err := repo.IncrementFailCountAndStopIfThreshold(ctx, residual.ID, 3, "auth_fail"); err != nil {
			t.Fatalf("increment: %v", err)
		}
		got, err := repo.Get(ctx, residual.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Status != models.CredentialStatusTempUnsched {
			t.Fatalf("Status = %q, want temp_unsched", got.Status)
		}
		if got.CooldownReason != "auth_fail" {
			t.Fatalf("CooldownReason = %q, want auth_fail", got.CooldownReason)
		}
		if got.CooldownUntil != nil {
			t.Fatalf("CooldownUntil != nil, want nil")
		}
	})
}
