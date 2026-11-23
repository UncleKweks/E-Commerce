package response

import "testing"

func TestNewPageMeta(t *testing.T) {
	cases := []struct {
		total     int64
		size      int
		wantPages int
	}{
		{0, 20, 0},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{57, 20, 3},
	}
	for _, tc := range cases {
		m := NewPageMeta(1, tc.size, tc.total)
		if m.TotalPages != tc.wantPages {
			t.Errorf("total=%d size=%d: TotalPages = %d, want %d", tc.total, tc.size, m.TotalPages, tc.wantPages)
		}
	}
}
