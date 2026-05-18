package providers

import (
	"context"
	"testing"

	"ghostcli/internal/engine/protocol"
)

// mockProvider is a test implementation of the Provider interface
type mockProvider struct {
	name             string
	supportsTools    bool
	supportsThinking bool
	modelMap         map[string]string
}

func (m *mockProvider) StreamChat(ctx context.Context, req *protocol.UnifiedChatRequest) (<-chan protocol.UnifiedStreamEvent, error) {
	ch := make(chan protocol.UnifiedStreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) SupportsTools() bool {
	return m.supportsTools
}

func (m *mockProvider) SupportsThinking() bool {
	return m.supportsThinking
}

func (m *mockProvider) MapModel(anthropicModel string) string {
	if mapped, ok := m.modelMap[anthropicModel]; ok {
		return mapped
	}
	return anthropicModel
}

// TestProviderInterface verifies that the Provider interface can be implemented
func TestProviderInterface(t *testing.T) {
	// Create a mock provider
	provider := &mockProvider{
		name:             "test-provider",
		supportsTools:    true,
		supportsThinking: false,
		modelMap: map[string]string{
			"claude-3-5-sonnet": "test-model-v1",
		},
	}

	// Verify interface compliance
	var _ Provider = provider

	// Test Name method
	if got := provider.Name(); got != "test-provider" {
		t.Errorf("Name() = %v, want %v", got, "test-provider")
	}

	// Test SupportsTools method
	if got := provider.SupportsTools(); got != true {
		t.Errorf("SupportsTools() = %v, want %v", got, true)
	}

	// Test SupportsThinking method
	if got := provider.SupportsThinking(); got != false {
		t.Errorf("SupportsThinking() = %v, want %v", got, false)
	}

	// Test MapModel method with existing mapping
	if got := provider.MapModel("claude-3-5-sonnet"); got != "test-model-v1" {
		t.Errorf("MapModel(claude-3-5-sonnet) = %v, want %v", got, "test-model-v1")
	}

	// Test MapModel method with non-existing mapping (should return original)
	if got := provider.MapModel("unknown-model"); got != "unknown-model" {
		t.Errorf("MapModel(unknown-model) = %v, want %v", got, "unknown-model")
	}

	// Test StreamChat method
	ctx := context.Background()
	req := &protocol.UnifiedChatRequest{
		Model:     "test-model",
		MaxTokens: 100,
		Stream:    true,
	}

	ch, err := provider.StreamChat(ctx, req)
	if err != nil {
		t.Errorf("StreamChat() error = %v, want nil", err)
	}

	// Verify channel is closed (mock implementation closes immediately)
	_, ok := <-ch
	if ok {
		t.Error("StreamChat() channel should be closed")
	}
}

// TestProviderInterfaceWithContext verifies context cancellation handling
func TestProviderInterfaceWithContext(t *testing.T) {
	provider := &mockProvider{
		name: "test-provider",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := &protocol.UnifiedChatRequest{
		Model:  "test-model",
		Stream: true,
	}

	// Should still be able to call StreamChat with cancelled context
	// (actual cancellation handling is provider-specific)
	_, err := provider.StreamChat(ctx, req)
	if err != nil {
		t.Errorf("StreamChat() with cancelled context error = %v, want nil", err)
	}
}
