package data

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
    ErrMsgViolateUniqueConstraint = "duplicate key value violates unique constraint"

    ErrRecordNotFound = errors.New("record not found")
    ErrEditConflict   = errors.New("edit conflict")
)

// Models puts models together in one struct.
type Models struct {
    Movie MovieModel
    User  UserModel
}

// NewModels returns a Models struct containing the initialized models.
func NewModels(p *pgxpool.Pool) Models {
    return Models{
        Movie: MovieModel{DB: p},
        User:  UserModel{DB: p},
    }
}
