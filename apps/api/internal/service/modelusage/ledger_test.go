package modelusage

import (
	"regexp"
	"testing"
)

func TestNewIDIdentifiesOneCall(t *testing.T) {
	uuidV4 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	first, second := NewID(), NewID()
	if first == second || !uuidV4.MatchString(first) || !uuidV4.MatchString(second) {
		t.Fatalf("expected distinct UUID v4 call IDs, got %q and %q", first, second)
	}
}
