package pagination

import "strconv"

const (
	DefaultPage  = 1
	DefaultLimit = 10
	MaxLimit     = 100
)

type Params struct {
	Page   int `json:"page"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func Parse(pageRaw, limitRaw string) Params {
	page, err := strconv.Atoi(pageRaw)
	if err != nil || page < 1 {
		page = DefaultPage
	}

	limit, err := strconv.Atoi(limitRaw)
	if err != nil || limit < 1 || limit > MaxLimit {
		limit = DefaultLimit
	}

	return Params{
		Page:   page,
		Limit:  limit,
		Offset: (page - 1) * limit,
	}
}
