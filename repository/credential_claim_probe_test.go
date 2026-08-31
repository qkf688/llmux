package repository

import (
	"context"
	"testing"
	"time"

	"github.com/qkf688/llmux/models"
)

func TestClaimLastProbeAt(t *testing.T) {
	ctx := context.Background()
	repo := NewCredentialRepo(newCredentialTestDB(t))
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-60 * time.Second)

	t.Run("nil last_probe_at claims", func(t *testing.T) {
		cred := models.Credential{Key: "k", KeyHash: "h", Status: models.CredentialStatusTempUnsched}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		n, err := repo.ClaimLastProbeAt(ctx, cred.ID, now, cutoff)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if n != 1 {
			t.Fatalf("rows = %d, want 1", n)
		}
		got, err := repo.Get(ctx, cred.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.LastProbeAt == nil || !got.LastProbeAt.Equal(now) {
			t.Fatalf("LastProbeAt = %v, want %v", got.LastProbeAt, now)
		}
	})

	t.Run("within interval rejects", func(t *testing.T) {
		recent := now.Add(-10 * time.Second)
		cred := models.Credential{
			Key: "k2", KeyHash: "h2", Status: models.CredentialStatusTempUnsched, LastProbeAt: &recent,
		}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		n, err := repo.ClaimLastProbeAt(ctx, cred.ID, now, cutoff)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if n != 0 {
			t.Fatalf("rows = %d, want 0（间隔内）", n)
		}
		got, err := repo.Get(ctx, cred.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.LastProbeAt == nil || !got.LastProbeAt.Equal(recent) {
			t.Fatalf("LastProbeAt changed to %v, want kept %v", got.LastProbeAt, recent)
		}
	})

	t.Run("past interval claims", func(t *testing.T) {
		old := now.Add(-2 * time.Minute)
		cred := models.Credential{
			Key: "k3", KeyHash: "h3", Status: models.CredentialStatusTempUnsched, LastProbeAt: &old,
		}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		n, err := repo.ClaimLastProbeAt(ctx, cred.ID, now, cutoff)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if n != 1 {
			t.Fatalf("rows = %d, want 1", n)
		}
	})

	t.Run("second claim within interval rejects", func(t *testing.T) {
		cred := models.Credential{Key: "k4", KeyHash: "h4", Status: models.CredentialStatusTempUnsched}
		if err := repo.Create(ctx, &cred); err != nil {
			t.Fatalf("create: %v", err)
		}
		n1, err := repo.ClaimLastProbeAt(ctx, cred.ID, now, cutoff)
		if err != nil {
			t.Fatalf("first claim: %v", err)
		}
		if n1 != 1 {
			t.Fatalf("first rows = %d, want 1", n1)
		}
		later := now.Add(time.Second)
		n2, err := repo.ClaimLastProbeAt(ctx, cred.ID, later, later.Add(-60*time.Second))
		if err != nil {
			t.Fatalf("second claim: %v", err)
		}
		if n2 != 0 {
			t.Fatalf("second rows = %d, want 0（已 claim）", n2)
		}
	})
}
