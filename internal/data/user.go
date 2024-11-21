package data

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"greenlight.zzh.net/internal/validator"
)

var ErrDuplicateEmail = errors.New("duplicate email")

// User represents an individual user.
type User struct {
    ID        int64     `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Password  password  `json:"-"`
    Activated bool      `json:"activated"`
    Version   int       `json:"version"`
}

type password struct {
    // The plaintext field is a *pointer* to a string, so that we're able to distinguish between
    // a password not provided at all, versus a password which is in fact the empty string "".
    plaintext *string
    hash      []byte
}

// Set calculates the bcrypt hash of a plaintext password and stores both the
// hash and the plaintext versions in the p struct.
func (p *password) Set(plaintext string) error {
    hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), 12)
    if err != nil {
        return err
    }

    p.plaintext = &plaintext
    p.hash = hash

    return nil
}

// Matches checks whether the provided plaintext password matches the hashed password stored
// in the struct, and returns true if it does.
func (p *password) Matches(plaintext string) (bool, error) {
    err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintext))
    if err != nil {
        switch {
        case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
            return false, nil
        default:
            return false, err
        }
    }

    return true, nil
}

// ValidateEmail validates an email address using validator v.
func ValidateEmail(v *validator.Validator, email string) {
    v.Check(email != "", "email", "must be provided")
    v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

// ValidatePassword validates a password using validator v.
func ValidatePassword(v *validator.Validator, password string) {
    v.Check(password != "", "password", "must be provided")
    v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
    v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

// ValidateUser validates the fields of user using validator v.
func ValidateUser(v *validator.Validator, user *User) {
    v.Check(user.Name != "", "name", "must be provided")
    v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

    ValidateEmail(v, user.Email)

    if user.Password.plaintext != nil {
        ValidatePassword(v, *user.Password.plaintext)
    }

    // If the password hash is nil, this will be due to a logic error in our codebase (probably
    // because we forgot to set a password for the user). It's a useful sanity check to include
    // here, but it's not a problem with the data provided by the client. So rather than adding
    // an error to the validation map we raise a panic instead.
    if user.Password.hash == nil {
        panic("missing hashed password for user")
    }
}

// UserModel struct wraps a database connection pool wrapper.
type UserModel struct {
    DB *PoolWrapper
}

// Insert inserts a new record in the users table.
func (m UserModel) Insert(user *User) error {
    query := `INSERT INTO users (name, email, password_hash, activated) 
              VALUES ($1, $2, $3, $4) 
              RETURNING id, created_at, version`

    args := []any{user.Name, user.Email, user.Password.hash, user.Activated}

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    err := m.DB.Pool.QueryRow(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
    if err != nil {
        switch {
        case strings.Contains(err.Error(), ErrMsgViolateUniqueConstraint) && strings.Contains(err.Error(), "email"):
            return ErrDuplicateEmail
        default:
            return err
        }
    }

    return nil
}

// GetByEmail retrives a user from the users table based on its email address.
func (m UserModel) GetByEmail(email string) (*User, error) {
    query := `SELECT id, created_at, name, email, password_hash, activated, version 
                FROM users 
               WHERE email = $1`

    var user User

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    err := m.DB.Pool.QueryRow(ctx, query, email).Scan(
        &user.ID,
        &user.CreatedAt,
        &user.Name,
        &user.Email,
        &user.Password.hash,
        &user.Activated,
        &user.Version,
    )

    if err != nil {
        switch {
        case errors.Is(err, pgx.ErrNoRows):
            return nil, ErrRecordNotFound
        default:
            return nil, err
        }
    }

    return &user, nil
}

// Update updates a record in the users table.
func (m UserModel) Update(user *User) error {
    query := `UPDATE users 
              SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1 
              WHERE id = $5 AND version = $6 
              RETURNING version`

    args := []any{
        user.Name,
        user.Email,
        user.Password.hash,
        user.Activated,
        user.ID,
        user.Version,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    err := m.DB.Pool.QueryRow(ctx, query, args...).Scan(&user.Version)
    if err != nil {
        switch {
            case strings.Contains(err.Error(), ErrMsgViolateUniqueConstraint) && strings.Contains(err.Error(), "email"):
                return ErrDuplicateEmail
            case errors.Is(err, pgx.ErrNoRows):
                return ErrEditConflict
            default:
                return err
        }
    }

    return nil
}