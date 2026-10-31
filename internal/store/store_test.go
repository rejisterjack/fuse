package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("FUSE_TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("FUSE_DATABASE_URL")
	}
	if url == "" {
		t.Skip("set FUSE_TEST_DATABASE_URL to run store tests")
	}
	ctx := context.Background()
	s, err := Connect(ctx, url)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUsageRuleAlertRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	soft, hard := 1.0, 2.0
	r, err := s.UpsertRule(ctx, BudgetRule{
		ScopeType: "agent",
		ScopeKey:  "test-agent",
		SoftUSD:   &soft,
		HardUSD:   &hard,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if r.ID == 0 {
		t.Fatal("expected id")
	}

	if err := s.InsertUsage(ctx, UsageEvent{
		Agent: "test-agent", User: "u", Task: "t", Model: "gpt-4o-mini",
		PromptTokens: 100, CompletionTokens: 50, CostUSD: 0.01,
	}); err != nil {
		t.Fatalf("insert usage: %v", err)
	}

	sum, err := s.SumSpend(ctx, "test-agent", "u", "t", time.Now().Add(-time.Hour))
	if err != nil || sum < 0.01 {
		t.Fatalf("sum=%v err=%v", sum, err)
	}

	if err := s.InsertAlert(ctx, "agent", "test-agent", "soft", "test"); err != nil {
		t.Fatalf("alert: %v", err)
	}
	ok, err := s.HasRecentAlert(ctx, "agent", "test-agent", "soft", time.Hour)
	if err != nil || !ok {
		t.Fatalf("HasRecentAlert ok=%v err=%v", ok, err)
	}

	rules, err := s.RulesForScope(ctx, "test-agent", "u", "t")
	if err != nil || len(rules) == 0 {
		t.Fatalf("rules=%v err=%v", rules, err)
	}

	_ = s.DeleteRule(ctx, r.ID)
}

func TestProxyKeyLifecycle(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	plain := "fuse_testkey_abcdefghijklmnopqrstuvwxyz"
	k, err := s.InsertProxyKey(ctx, plain, "bot", "user1", "demo")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := s.LookupProxyKey(ctx, plain)
	if err != nil || got.Agent != "bot" {
		t.Fatalf("lookup=%+v err=%v", got, err)
	}
	if err := s.RevokeProxyKey(ctx, k.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := s.LookupProxyKey(ctx, plain); err == nil {
		t.Fatal("expected lookup fail after revoke")
	}
}
