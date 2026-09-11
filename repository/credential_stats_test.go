package repository

import (
	"context"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

// 凭据终身统计原子自增（请求/错误/最近使用）：chat 尝试记账的落库原语。
// 契约——只动自己的列，软删行不写（RowsAffected=0），与
// IncrementFailCountAndStopIfThreshold 同属"单语句无读回"家族。
func TestRecordCredentialStats(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newCredentialTestDB(t))

	t.Run("success increments total_requests and touches last_used_at", func(t *testing.T) {
		cred := models.Credential{Key: "s1", KeyHash: "s1", Status: models.CredentialStatusActive}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		at := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
		for i := int64(1); i <= 2; i++ {
			n, err := repo.RecordCredentialSuccess(ctx, cred.ID, at)
			if err != nil {
				t.Fatalf("success step %d: %v", i, err)
			}
			if n != 1 {
				t.Fatalf("rows step %d = %d, want 1", i, n)
			}
			got, err := repo.Get(ctx, cred.ID)
			if err != nil {
				t.Fatalf("get step %d: %v", i, err)
			}
			if got.TotalRequests != i {
				t.Fatalf("TotalRequests step %d = %d, want %d", i, got.TotalRequests, i)
			}
			if got.LastUsedAt == nil || !got.LastUsedAt.Equal(at) {
				t.Fatalf("LastUsedAt step %d = %v, want %v", i, got.LastUsedAt, at)
			}
			if got.TotalErrors != 0 {
				t.Fatalf("TotalErrors = %d, want 0（成功写不得碰错误列）", got.TotalErrors)
			}
			if got.FailCount != 0 {
				t.Fatalf("FailCount = %d, want 0（成功写不得碰连败列）", got.FailCount)
			}
		}
	})

	t.Run("error increments total_errors only", func(t *testing.T) {
		cred := models.Credential{Key: "e1", KeyHash: "e1", Status: models.CredentialStatusActive}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		n, err := repo.RecordCredentialError(ctx, cred.ID)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if n != 1 {
			t.Fatalf("rows = %d, want 1", n)
		}
		got, err := repo.Get(ctx, cred.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.TotalErrors != 1 {
			t.Fatalf("TotalErrors = %d, want 1", got.TotalErrors)
		}
		if got.TotalRequests != 0 {
			t.Fatalf("TotalRequests = %d, want 0（错误写不得碰请求列）", got.TotalRequests)
		}
		if got.LastUsedAt != nil {
			t.Fatalf("LastUsedAt = %v, want nil（失败不刷新最近使用）", got.LastUsedAt)
		}
	})

	t.Run("does not write soft-deleted rows", func(t *testing.T) {
		cred := models.Credential{Key: "d1", KeyHash: "d1", Status: models.CredentialStatusActive}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := repo.Delete(ctx, cred.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		n, err := repo.RecordCredentialSuccess(ctx, cred.ID, time.Now())
		if err != nil {
			t.Fatalf("success on deleted: %v", err)
		}
		if n != 0 {
			t.Fatalf("success rows = %d, want 0（软删行不落统计）", n)
		}
		n, err = repo.RecordCredentialError(ctx, cred.ID)
		if err != nil {
			t.Fatalf("error on deleted: %v", err)
		}
		if n != 0 {
			t.Fatalf("error rows = %d, want 0（软删行不落统计）", n)
		}
	})
}
