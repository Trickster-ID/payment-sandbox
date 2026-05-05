// Branch map for Parse:
// ├── pageRaw parse fails                -> page = DefaultPage        [Case 1]
// ├── pageRaw parses, page < 1 (0)       -> page = DefaultPage        [Case 2]
// ├── pageRaw parses, page < 1 (-5)      -> page = DefaultPage        [Case 3]
// ├── limitRaw parse fails               -> limit = DefaultLimit      [Case 4]
// ├── limitRaw parses, limit < 1 (0)     -> limit = DefaultLimit      [Case 5]
// ├── limitRaw parses, limit < 1 (-5)    -> limit = DefaultLimit      [Case 6]
// ├── limitRaw parses, limit > MaxLimit   -> limit = DefaultLimit      [Case 7]
// └── all valid, all limits OK            -> success                  [Case 8]
package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	type args struct {
		pageRaw  string
		limitRaw string
	}
	type wants struct {
		page   int
		limit  int
		offset int
	}

	tests := []struct {
		name  string
		args  args
		wants wants
	}{
		{
			name:  "1. pageRaw parse fails -> page = DefaultPage",
			args:  args{pageRaw: "not-a-number", limitRaw: "20"},
			wants: wants{page: DefaultPage, limit: 20, offset: 0},
		},
		{
			name:  "2. pageRaw parses to 0 (page < 1) -> page = DefaultPage",
			args:  args{pageRaw: "0", limitRaw: "20"},
			wants: wants{page: DefaultPage, limit: 20, offset: 0},
		},
		{
			name:  "3. pageRaw parses to -5 (page < 1) -> page = DefaultPage",
			args:  args{pageRaw: "-5", limitRaw: "20"},
			wants: wants{page: DefaultPage, limit: 20, offset: 0},
		},
		{
			name:  "4. limitRaw parse fails -> limit = DefaultLimit",
			args:  args{pageRaw: "2", limitRaw: "not-a-number"},
			wants: wants{page: 2, limit: DefaultLimit, offset: DefaultLimit},
		},
		{
			name:  "5. limitRaw parses to 0 (limit < 1) -> limit = DefaultLimit",
			args:  args{pageRaw: "2", limitRaw: "0"},
			wants: wants{page: 2, limit: DefaultLimit, offset: DefaultLimit},
		},
		{
			name:  "6. limitRaw parses to -5 (limit < 1) -> limit = DefaultLimit",
			args:  args{pageRaw: "2", limitRaw: "-5"},
			wants: wants{page: 2, limit: DefaultLimit, offset: DefaultLimit},
		},
		{
			name:  "7. limitRaw > MaxLimit -> limit = DefaultLimit",
			args:  args{pageRaw: "1", limitRaw: "101"},
			wants: wants{page: 1, limit: DefaultLimit, offset: 0},
		},
		{
			name:  "8. all valid, page >= 1 and 1 <= limit <= MaxLimit -> success",
			args:  args{pageRaw: "3", limitRaw: "50"},
			wants: wants{page: 3, limit: 50, offset: 100},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := Parse(tt.args.pageRaw, tt.args.limitRaw)

			assert.Equal(t, tt.wants.page, result.Page, "page")
			assert.Equal(t, tt.wants.limit, result.Limit, "limit")
			assert.Equal(t, tt.wants.offset, result.Offset, "offset")
		})
	}
}
