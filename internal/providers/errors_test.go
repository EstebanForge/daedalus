package providers

import "testing"

func TestEncodeDecodeEventErrorRoundTrip(t *testing.T) {
	t.Parallel()

	encoded := EncodeEventError(ProviderError{
		Category: ErrorRateLimit,
		Message:  "rate limited",
	})
	decoded := DecodeEventError(encoded)
	providerErr, ok := decoded.(ProviderError)
	if !ok {
		t.Fatalf("expected ProviderError, got %T", decoded)
	}
	if providerErr.Category != ErrorRateLimit {
		t.Fatalf("unexpected category: %s", providerErr.Category)
	}
	if providerErr.Message != "rate limited" {
		t.Fatalf("unexpected message: %q", providerErr.Message)
	}
}
