package backup

import (
	"testing"

	"devclubxnst.com/cadence/internal/config"
)

func TestNewDisabledWithoutR2Config(t *testing.T) {
	svc, err := New(&config.Config{}, nil, nil)
	if err != nil || svc != nil {
		t.Fatalf("expected nil service and nil error, got %v, %v", svc, err)
	}
}

func TestNewEnabledDerivesEndpoint(t *testing.T) {
	cfg := &config.Config{
		R2AccountID: "abc", R2AccessKeyID: "k", R2SecretAccessKey: "s", R2Bucket: "b",
		BackupInterval: 1, BackupPrefix: "/p/",
	}
	svc, err := New(cfg, nil, nil)
	if err != nil || svc == nil {
		t.Fatalf("expected service, got %v, %v", svc, err)
	}
	if svc.prefix != "p" {
		t.Fatalf("prefix not trimmed: %q", svc.prefix)
	}
	if got := cfg.BackupEndpoint(); got != "abc.r2.cloudflarestorage.com" {
		t.Fatalf("endpoint = %q", got)
	}
}
