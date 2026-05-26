package types

import "testing"

func TestLearnedRecipeMaskRoundTrip(t *testing.T) {
	var flags [LearnedRecipeFlagCount]bool
	for _, id := range []int{1, 31, 32, 127, 575} {
		flags[id] = true
	}

	encoded := EncodeLearnedRecipeMask(flags)
	if len(encoded) != LearnedRecipeFlagCount/8 {
		t.Fatalf("encoded length = %d, want %d", len(encoded), LearnedRecipeFlagCount/8)
	}
	decoded := DecodeLearnedRecipeMask(encoded)
	if decoded != flags {
		t.Fatal("decoded recipe mask does not match original flags")
	}
	if LearnedRecipeCount(decoded) != 5 {
		t.Fatalf("recipe count = %d, want 5", LearnedRecipeCount(decoded))
	}
}

func TestDecodeLearnedRecipeRawFlags(t *testing.T) {
	raw := make([]byte, LearnedRecipeFlagCount)
	raw[2] = 1
	raw[300] = 1

	decoded := DecodeLearnedRecipeMask(raw)
	if !decoded[2] || !decoded[300] {
		t.Fatal("raw 576-byte recipe flags were not decoded")
	}
}
