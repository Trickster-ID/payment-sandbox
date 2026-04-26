package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		pageRaw   string
		limitRaw  string
		wantPage  int
		wantLimit int
		wantOff   int
	}{
		{
			name:      "valid values",
			pageRaw:   "2",
			limitRaw:  "20",
			wantPage:  2,
			wantLimit: 20,
			wantOff:   20,
		},
		{
			name:      "default page when invalid",
			pageRaw:   "x",
			limitRaw:  "20",
			wantPage:  1,
			wantLimit: 20,
			wantOff:   0,
		},
		{
			name:      "default limit when invalid",
			pageRaw:   "2",
			limitRaw:  "x",
			wantPage:  2,
			wantLimit: 10,
			wantOff:   10,
		},
		{
			name:      "default limit when out of range",
			pageRaw:   "1",
			limitRaw:  "999",
			wantPage:  1,
			wantLimit: 10,
			wantOff:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.pageRaw, tc.limitRaw)
			assert.Equal(t, tc.wantPage, got.Page)
			assert.Equal(t, tc.wantLimit, got.Limit)
			assert.Equal(t, tc.wantOff, got.Offset)
		})
	}
}
