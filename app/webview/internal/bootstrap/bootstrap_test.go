package bootstrap

import (
	"context"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestBootstrap_BootAndShutdown(t *testing.T) {
	tempDir := t.TempDir()

	appInstance, err := Boot(BootOptions{
		DataDir: tempDir,
	})
	if err != nil {
		t.Fatalf("expected Boot() to succeed, got: %v", err)
	}
	if appInstance == nil {
		t.Fatal("expected non-nil app instance")
	}

	// Verify ServiceStartup and cancellation lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = appInstance.ServiceStartup(ctx, application.ServiceOptions{})
	if err != nil {
		t.Errorf("ServiceStartup failed: %v", err)
	}

	// Test graceful shutdown
	appInstance.ServiceShutdown()
}
