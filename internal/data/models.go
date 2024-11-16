package data

import (
	"database/sql"
	"errors"
)

var (
    ErrRecordNotFound = errors.New("record not found")
    ErrEditConflict   = errors.New("edit conflict")
)

// Models puts models together in one struct.
type Models struct {
    Movie MovieModel
}

// NewModels returns a Models struct containing the initialized models.
func NewModels(db *sql.DB) Models {
    return Models{
        Movie: MovieModel{DB: db},
    }
}
