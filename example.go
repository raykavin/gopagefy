package gopagefy

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Domain entity

type Transaction struct {
	ID          uint    `json:"id"           gorm:"primaryKey"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Status      string  `json:"status"`
}

// Request DTO

type ListTransactionsRequest struct {
	Page
	Status    string `form:"status"`
	MinAmount string `form:"min_amount"`
	Sort      string `form:"sort"` // e.g. "created_at desc,amount asc"
}

// Handler

type TransactionHandler struct {
	db *gorm.DB
}

func (h *TransactionHandler) List(c *gin.Context) {
	var req ListTransactionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build filters fluently
	fb := NewFilterBuilder().
		WhereIf(req.Status != "", "status", Eq, req.Status)

	if req.MinAmount != "" {
		if v, err := strconv.ParseFloat(req.MinAmount, 64); err == nil {
			fb.Where("amount", Gte, v)
		}
	}

	// Build sorts from query string
	sorts := ParseSort(req.Sort)
	if len(sorts) == 0 {
		sorts = NewSortBuilder().
			OrderBy("created_at", Desc).
			Build()
	}

	query := NewQuery(req.Page, fb.Build(), sorts)

	// Execute with GORM scope
	var rows []Transaction
	var total int64

	h.db.Model(&Transaction{}).
		Scopes(Scope(query, &total)).
		Find(&rows)

	result := NewResult(rows, int(total), query.Page)
	c.JSON(http.StatusOK, result)
}

// Routes

func main() {
	r := gin.Default()

	db, _ := gorm.Open(gorm.Config{
		// Your dial configuration
	})
	h := &TransactionHandler{db: db}

	r.GET("/transactions", h.List)
	err := r.Run(":8080")
	if err != nil {
		panic(err)
	}
}
