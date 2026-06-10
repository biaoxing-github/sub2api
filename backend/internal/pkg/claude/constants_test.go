package claude

import "testing"

func TestDefaultModelsIncludesClaudeFable5(t *testing.T) {
	ids := DefaultModelIDs()
	for _, id := range ids {
		if id == "claude-fable-5" {
			return
		}
	}

	t.Fatalf("DefaultModelIDs() does not include claude-fable-5: %v", ids)
}
