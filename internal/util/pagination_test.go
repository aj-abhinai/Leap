package util

import "testing"

func TestOffsetClampsPage(t *testing.T) {
	tests := []struct {
		name    string
		page    int
		perPage int
		want    int
	}{
		{name: "first page", page: 1, perPage: 20, want: 0},
		{name: "second page", page: 2, perPage: 20, want: 20},
		{name: "zero page becomes first", page: 0, perPage: 20, want: 0},
		{name: "negative page becomes first", page: -5, perPage: 20, want: 0},
		{name: "zero per page is treated as one", page: 3, perPage: 0, want: 2},
		{name: "huge page is clamped", page: 1<<63 - 1, perPage: 200, want: (maxPage - 1) * 200},
		{name: "huge per page is clamped", page: 2, perPage: 1 << 62, want: maxPage},
		{name: "huge negative page", page: -1 << 63, perPage: 200, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Offset(tt.page, tt.perPage); got != tt.want {
				t.Errorf("Offset(%d, %d) = %d, want %d", tt.page, tt.perPage, got, tt.want)
			}
		})
	}
}

func TestOffsetNeverOverflows(t *testing.T) {
	for _, tt := range []struct {
		page    int
		perPage int
	}{
		{page: 1<<63 - 1, perPage: 1<<63 - 1},
		{page: maxPage, perPage: 1 << 62},
		{page: 1, perPage: 1 << 62},
		{page: -1 << 63, perPage: -1 << 63},
	} {
		if got := Offset(tt.page, tt.perPage); got < 0 {
			t.Errorf("Offset(%d, %d) overflowed to negative: %d", tt.page, tt.perPage, got)
		}
	}
}
