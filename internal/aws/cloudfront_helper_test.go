package awsclient

import (
	"testing"
	"time"
)

func TestDeduplicateKeys(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		newID    string
		want     []string
	}{
		{
			name:     "new key first, no duplicates",
			existing: []string{"b", "c"},
			newID:    "a",
			want:     []string{"a", "b", "c"},
		},
		{
			name:     "new key already in existing — deduped",
			existing: []string{"a", "b"},
			newID:    "a",
			want:     []string{"a", "b"},
		},
		{
			name:     "empty existing",
			existing: nil,
			newID:    "x",
			want:     []string{"x"},
		},
		{
			name:     "duplicates within existing are preserved (only new deduped)",
			existing: []string{"b", "b", "c"},
			newID:    "a",
			want:     []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateKeys(tt.existing, tt.newID)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPartitionByAge(t *testing.T) {
	now := time.Now()
	metas := []keyMeta{
		{id: "old", createdAt: now.Add(-3 * 24 * time.Hour)},
		{id: "new", createdAt: now},
		{id: "mid", createdAt: now.Add(-24 * time.Hour)},
	}

	t.Run("limit=2 keeps newest 2, evicts oldest", func(t *testing.T) {
		keep, evict := partitionByAge(metas, 2)
		if len(keep) != 2 {
			t.Fatalf("keep len %d, want 2", len(keep))
		}
		if keep[0] != "new" {
			t.Errorf("keep[0] = %q, want new", keep[0])
		}
		if keep[1] != "mid" {
			t.Errorf("keep[1] = %q, want mid", keep[1])
		}
		if len(evict) != 1 || evict[0] != "old" {
			t.Errorf("evict = %v, want [old]", evict)
		}
	})

	t.Run("limit >= len keeps all", func(t *testing.T) {
		keep, evict := partitionByAge(metas, 10)
		if len(keep) != 3 {
			t.Errorf("keep len %d, want 3", len(keep))
		}
		if len(evict) != 0 {
			t.Errorf("evict len %d, want 0", len(evict))
		}
	})

	t.Run("limit=0 evicts all", func(t *testing.T) {
		keep, evict := partitionByAge(metas, 0)
		if len(keep) != 0 {
			t.Errorf("keep len %d, want 0", len(keep))
		}
		if len(evict) != 3 {
			t.Errorf("evict len %d, want 3", len(evict))
		}
	})
}
