package utils

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaginationParams represents pagination parameters
type PaginationParams struct {
	Page    int
	PerPage int
	OrderBy string
	Sort    string
}

// GetPaginationParams extracts pagination parameters from request
func GetPaginationParams(c *gin.Context) PaginationParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))
	orderBy := c.DefaultQuery("order_by", "id")
	sort := c.DefaultQuery("sort", "desc")

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	return PaginationParams{
		Page:    page,
		PerPage: perPage,
		OrderBy: orderBy,
		Sort:    sort,
	}
}

// Paginate applies pagination to a GORM query
func Paginate(params PaginationParams) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset := (params.Page - 1) * params.PerPage
		orderClause := params.OrderBy + " " + params.Sort
		return db.Offset(offset).Limit(params.PerPage).Order(orderClause)
	}
}

// BuildPaginationMeta builds pagination metadata
func BuildPaginationMeta(page, perPage int, total int64) PaginationMeta {
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))

	return PaginationMeta{
		CurrentPage: page,
		PerPage:     perPage,
		Total:       total,
		TotalPages:  totalPages,
	}
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       interface{}    `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}
