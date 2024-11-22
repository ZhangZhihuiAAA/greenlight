package data

import (
	"errors"
)

var (
    ErrMsgViolateUniqueConstraint = "duplicate key value violates unique constraint"

    ErrRecordNotFound = errors.New("record not found")
    ErrEditConflict   = errors.New("edit conflict")
)

// Models puts models together in one struct.
type Models struct {
    Movie MovieModel
    Token TokenModel
    User  UserModel
}

// NewModels returns a Models struct containing the initialized models.
func NewModels(pw *PoolWrapper) Models {
    return Models{
        Movie: MovieModel{DB: pw},
        Token: TokenModel{DB: pw},
        User:  UserModel{DB: pw},
    }
}