package modelmanager_test

import (
	"context"
	"testing"

	"github.com/sarthak/opentelemetry-tutorial/internal/pipeline/modelmanager"
	"go.opentelemetry.io/otel"
)

func TestModelManager_Lifecycle(t *testing.T) {
	ctx := context.Background()
	mgr, err := modelmanager.New("metal", otel.Meter("test"), otel.Tracer("test"))
	if err != nil {
		t.Fatalf("Failed to create model manager: %v", err)
	}

	if mgr.ActiveModel() != "" {
		t.Errorf("Expected active model to be empty, got %s", mgr.ActiveModel())
	}
	if mgr.AllocatedVRAM() != 0 {
		t.Errorf("Expected 0 VRAM, got %d", mgr.AllocatedVRAM())
	}

	// 1. Load llama model
	relLlama, err := mgr.LoadModel(ctx, "llama-3-8b.gguf", 4_500_000_000)
	if err != nil {
		t.Fatalf("Failed to load llama: %v", err)
	}
	if mgr.ActiveModel() != "llama-3-8b.gguf" {
		t.Errorf("Expected llama active, got %s", mgr.ActiveModel())
	}
	if mgr.AllocatedVRAM() != 4_500_000_000 {
		t.Errorf("Expected 4.5GB VRAM, got %d", mgr.AllocatedVRAM())
	}

	// 2. Unload llama model
	relLlama(ctx)
	if mgr.ActiveModel() != "" {
		t.Errorf("Expected active model empty after release, got %s", mgr.ActiveModel())
	}
	if mgr.AllocatedVRAM() != 0 {
		t.Errorf("Expected 0 VRAM after release, got %d", mgr.AllocatedVRAM())
	}

	// 3. Double release should be safe no-op
	relLlama(ctx)
	if mgr.AllocatedVRAM() != 0 {
		t.Errorf("Expected 0 VRAM after second release, got %d", mgr.AllocatedVRAM())
	}

	// 4. Test automatic eviction when loading another model without manual release
	_, err = mgr.LoadModel(ctx, "llama-3-8b.gguf", 4_500_000_000)
	if err != nil {
		t.Fatalf("Failed to load llama: %v", err)
	}

	relEmbed, err := mgr.LoadModel(ctx, "bge-large.gguf", 1_300_000_000)
	if err != nil {
		t.Fatalf("Failed to load bge: %v", err)
	}
	defer relEmbed(ctx)

	if mgr.ActiveModel() != "bge-large.gguf" {
		t.Errorf("Expected bge active after eviction, got %s", mgr.ActiveModel())
	}
	if mgr.AllocatedVRAM() != 1_300_000_000 {
		t.Errorf("Expected 1.3GB VRAM, got %d", mgr.AllocatedVRAM())
	}
}
