package data

import (
	"strings"

	"greenlight.zzh.net/internal/validator"
)

// Filter is used for filtering, sorting and pagination.
type Filter struct {
    Page         int
    PageSize     int
    Sort         string
    SortSafeList []string
}

// ValidateFilter validates the fields of f using validator v.
func ValidateFilter(v *validator.Validator, f Filter) {
    v.Check(f.Page > 0, "page", "must be greater than 0")
    v.Check(f.Page <= 10_000_000, "page", "must be less than or equal to 10000000")
    v.Check(f.PageSize > 0, "page_size", "must be greater than 0")
    v.Check(f.PageSize <= 100, "page_size", "must be less than or equal to 100")
    v.Check(validator.PermittedValue(f.Sort, f.SortSafeList...), "sort", "invalid sort value")
}

// sortColumn checks that the client-provided filed matches one of the entries in the safelist
// and if it does, extracts the column name from the Sort field by stripping the leading hyphen
// character (if one exists).
func (f Filter) sortColumn() string {
    for _, safeValue := range f.SortSafeList {
        if f.Sort == safeValue {
            return strings.TrimPrefix(f.Sort, "-")
        }
    }

    panic("unsafe sort parameter: " + f.Sort)
}

// sortDirection returns the sort direction ("ASC" or "DESC") depending on the
// prefix character of the Sort field.
func (f Filter) sortDirection() string {
    if strings.HasPrefix(f.Sort, "-") {
        return "DESC"
    }

    return "ASC"
}

func (f Filter) limit() int {
    return f.PageSize
}

func (f Filter) offset() int {
    return (f.Page - 1) * f.PageSize
}

// MetaData holds the pagination metadata.
type Metadata struct {
    CurrentPage  int `json:"current_page,omitempty"`
    PageSize     int `json:"page_size,omitempty"`
    FirstPage    int `json:"first_page,omitempty"`
    LastPage     int `json:"last_page,omitempty"`
    TotalRecords int `json:"total_records,omitempty"`
}

func calculateMetadata(totalRecords, page, pageSize int) Metadata {
    if totalRecords == 0 {
        return Metadata{}
    }

    // Note that when the last page value is calculated we are dividing two int values, and
    // when dividing integer types in Go the result will also be an integer type, with
    // the modulus (or remainder) dropped. So, for example, if there were 12 records in total
    // and a page size of 5, the last page value would be (12+5-1)/5 = 3.2, which is then
    // truncated to 3 by Go.
    return Metadata{
        CurrentPage:  page,
        PageSize:     pageSize,
        FirstPage:    1,
        LastPage:     (totalRecords + pageSize - 1) / pageSize,
        TotalRecords: totalRecords,
    }
}
