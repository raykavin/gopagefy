# gopagefy

[![Go Reference](https://pkg.go.dev/badge/github.com/raykavin/gopagefy.svg)](https://pkg.go.dev/github.com/raykavin/gopagefy)
[![Go Version](https://img.shields.io/badge/go-1.21+-blue)](https://golang.org/dl/)
[![Go Report Card](https://goreportcard.com/badge/github.com/raykavin/gopagefy)](https://goreportcard.com/report/github.com/raykavin/gopagefy)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE.md)

A generic, zero-dependency Go library for **pagination and filtering**. Provides type-safe paginated responses, a fluent filter builder, sort parsing, and a plug-and-play GORM adapter — all without boilerplate.

---

## Features

| | |
|---|---|
| 🔢 **Generic responses** | `Result[T]` works for any entity using Go generics |
| 🔍 **Fluent filter builder** | Compose `WHERE` clauses with a clean, chainable API |
| ↕️ **Sort builder** | Build and parse `ORDER BY` clauses from query strings |
| 🗃️ **GORM adapter** | Drop-in `Scope()` with automatic `COUNT` before `LIMIT/OFFSET` |
| 🛡️ **Safe defaults** | Normalizes page number, per-page, and enforces max limits |
| ⚡ **Zero dependencies** | Core package is pure Go; GORM adapter is opt-in |

---

## Installation

```bash
go get github.com/raykavin/gopagefy
```

Requires Go 1.21+.

---

## Quick Start

```go
package main

import (
    "fmt"

    "github.com/raykavin/gopagefy"
)

func main() {
    page := gopagefy.Page{Number: 1, PerPage: 10}

    filters := gopagefy.NewFilterBuilder().
        Where("status", gopagefy.Eq, "active").
        Where("amount", gopagefy.Gte, 100).
        Build()

    sorts := gopagefy.NewSortBuilder().
        OrderBy("created_at", gopagefy.Desc).
        Build()

    query := gopagefy.NewQuery(page, filters, sorts)

    // Simulate a paginated result
    items := []string{"item1", "item2", "item3"}
    result := gopagefy.NewResult(items, 42, query.Page)

    fmt.Println(result.Page)       // 1
    fmt.Println(result.TotalPages) // 5
    fmt.Println(result.HasNext)    // true
}
```

---

## Core Concepts

### Page

`Page` holds the pagination request and normalizes itself on demand.

```go
page := gopagefy.Page{Number: 2, PerPage: 20}
page.Normalize() // enforces defaults and MaxPerPage

fmt.Println(page.Offset()) // 20
```

Default values:

| Constant | Value |
|---|---|
| `DefaultPage` | 1 |
| `DefaultPerPage` | 20 |
| `MaxPerPage` | 100 |

### Result[T]

`Result[T]` is the generic paginated response. It works for any type — no type assertions needed.

```go
type Transaction struct {
    ID     uint
    Amount float64
}

rows  := []Transaction{{ID: 1, Amount: 99.9}}
total := 150

result := gopagefy.NewResult(rows, total, page)
// result.Data       → []Transaction
// result.Total      → 150
// result.TotalPages → 8
// result.HasNext    → true
// result.HasPrev    → true
```

JSON response shape:

```json
{
  "data": [...],
  "total": 150,
  "page": 2,
  "per_page": 20,
  "total_pages": 8,
  "has_next": true,
  "has_prev": true
}
```

### Query

`Query` aggregates `Page`, `[]Filter`, and `[]Sort` into a single object that travels through your application layers.

```go
query := gopagefy.NewQuery(page, filters, sorts)
// query.Page    → normalized Page
// query.Filters → []Filter
// query.Sorts   → []Sort
```

---

## Filtering

### FilterBuilder

Build `WHERE` conditions with a fluent, readable API.

```go
filters := gopagefy.NewFilterBuilder().
    Where("status", gopagefy.Eq, "active").
    Where("amount", gopagefy.Gte, 100).
    Where("deleted_at", gopagefy.IsNull, nil).
    WhereIf(userID != "", "user_id", gopagefy.Eq, userID). // only added if true
    Build()
```

### Supported Operators

| Operator | SQL |
|---|---|
| `Eq` | `=` |
| `Neq` | `<>` |
| `Gt` | `>` |
| `Gte` | `>=` |
| `Lt` | `<` |
| `Lte` | `<=` |
| `Like` | `LIKE '%value%'` |
| `ILike` | `ILIKE '%value%'` |
| `In` | `IN (?)` |
| `NotIn` | `NOT IN (?)` |
| `IsNull` | `IS NULL` |
| `IsNotNull` | `IS NOT NULL` |

### Conditional Filters

`WhereIf` adds the filter only when the condition is `true`, keeping the builder clean without extra `if` blocks.

```go
gopagefy.NewFilterBuilder().
    WhereIf(req.Status != "",    "status",     gopagefy.Eq,  req.Status).
    WhereIf(req.MinAmount > 0,   "amount",     gopagefy.Gte, req.MinAmount).
    WhereIf(req.Search != "",    "description",gopagefy.ILike, req.Search).
    Build()
```

---

## Sorting

### SortBuilder

```go
sorts := gopagefy.NewSortBuilder().
    OrderBy("created_at", gopagefy.Desc).
    OrderBy("name", gopagefy.Asc).
    Build()
```

### ParseSort

Parse a sort string directly from a query parameter (e.g. `?sort=name+asc,created_at+desc`):

```go
sorts := gopagefy.ParseSort("name asc,created_at desc")
// []Sort{
//   {Field: "name",       Direction: Asc},
//   {Field: "created_at", Direction: Desc},
// }
```

---

## GORM Integration

### Scope

`gopagefy.Scope` returns a GORM scope that applies filters, sorts, counts, and pagination in one call.

```go
var rows  []Transaction
var total int64

query := gopagefy.NewQuery(page, filters, sorts)

db.Model(&Transaction{}).
    Scopes(gopagefy.Scope(query, &total)).
    Find(&rows)

result := gopagefy.NewResult(rows, int(total), query.Page)
```

> The scope runs `COUNT(*)` **before** applying `LIMIT` and `OFFSET`, so `total` always reflects the full dataset.

---

## HTTP Handler Example (Gin)

```go
type ListRequest struct {
    gopagefy.Page
    Status    string `form:"status"`
    MinAmount string `form:"min_amount"`
    Sort      string `form:"sort"`
}

func (h *Handler) List(c *gin.Context) {
    var req ListRequest
    if err := c.ShouldBindQuery(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    minAmount, _ := strconv.ParseFloat(req.MinAmount, 64)

    filters := gopagefy.NewFilterBuilder().
        WhereIf(req.Status != "", "status", gopagefy.Eq, req.Status).
        WhereIf(minAmount > 0,    "amount", gopagefy.Gte, minAmount).
        Build()

    sorts := gopagefy.ParseSort(req.Sort)
    if len(sorts) == 0 {
        sorts = gopagefy.NewSortBuilder().
            OrderBy("created_at", gopagefy.Desc).
            Build()
    }

    query := gopagefy.NewQuery(req.Page, filters, sorts)

    var rows  []Transaction
    var total int64

    h.db.Model(&Transaction{}).
        Scopes(gopagefy.Scope(query, &total)).
        Find(&rows)

    c.JSON(http.StatusOK, gopagefy.NewResult(rows, int(total), query.Page))
}
```

---

## File Structure

```
gopagefy/
├── paginator.go   # Page, Query, Offset, Normalize
├── filter.go      # Filter, Sort, FilterBuilder, SortBuilder, ParseSort
├── paginator.go   # Result[T], NewResult[T]
└── gorm.go        # Scope() GORM adapter
```

---

## Best Practices

**Always call `Normalize()`** (or use `NewQuery()`) before using `Page`, otherwise defaults won't be applied.

```go
// ✅ Good
query := gopagefy.NewQuery(page, filters, sorts)

// ⚠️ Missing normalization
db.Limit(page.PerPage).Offset(page.Offset())
```

**Use `WhereIf` over manual conditionals** to keep your filter chains readable.

```go
// ✅ Good
fb.WhereIf(req.Status != "", "status", gopagefy.Eq, req.Status)

// ❌ Avoid
if req.Status != "" {
    fb.Where("status", gopagefy.Eq, req.Status)
}
```

**Pass `Query` across layers** instead of individual `Page`, `[]Filter`, `[]Sort` to keep function signatures clean.

```go
// ✅ Good
func (r *repo) List(ctx context.Context, q gopagefy.Query) ([]T, int, error)

// ❌ Avoid
func (r *repo) List(ctx context.Context, page int, perPage int, filters []Filter, ...) 
```

---

## Contributing

Contributions to gopagefy are welcome! Here are some ways you can help improve the project:

- **Report bugs and suggest features** by opening issues on GitHub
- **Submit pull requests** with bug fixes or new features
- **Improve documentation** to help other users and developers
- **Share your custom strategies** with the community

## License

gopagefy is distributed under the **MIT License**.  
For complete license terms and conditions, see the [LICENSE](LICENSE.md) file in the repository.

---

## Contact

For support, collaboration, or questions about gopagefy:

**Email**: [raykavin.meireles@gmail.com](mailto:raykavin.meireles@gmail.com)  
**GitHub**: [@raykavin](https://github.com/raykavin)  