package response

type PaginationMeta struct {
	Page  int `json:"page" example:"1"`
	Limit int `json:"limit" example:"10"`
	Total int `json:"total" example:"42"`
}
