package metrics

import "testing"

func TestCollectAllNoTrackedPIDs(t *testing.T) {
	c := NewCollector()
	c.CollectAll()

	if got := len(c.GetAll()); got != 0 {
		t.Fatalf("GetAll() len = %d, want 0", got)
	}
}
