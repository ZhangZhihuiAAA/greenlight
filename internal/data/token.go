package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"time"

	"greenlight.zzh.net/internal/validator"
)

const ScopeActivation = "activation"

// Token holds the data for a token.
type Token struct {
    Plaintext string
    Hash      []byte
    UserID    int64
    Expiry    time.Time
    Scope     string
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
    // We add the provided ttl (time-to-live) duration parameter to the current time
    // to get the expiry time.
    token := &Token{
        UserID: userID,
        Expiry: time.Now().Add(ttl),
        Scope:  scope,
    }

    // Initialize a zero-valued byte slice with a length of 16 bytes.
    randomBytes := make([]byte, 16)

    // Use the Read() function from the crypto/rand package to fill the byte slice with random 
    // bytes from your operating system's CSPRNG. This will return an error if the CSPRNG fails 
    // to function correctly.
    _, err := rand.Read(randomBytes)
    if err != nil {
        return nil, err
    }

    // Encode the byte slice to a base32-encoded string and assign it to the token's Plaintext 
    // field. This will be the token string that we send to the user in their welcome email. 
    // They will look similar to this: Y3QMGX3PJ3WLRL2YRTQGQ6KRHU
    // Note that by default base32 strings may be padded at the end with the = character. We dont' 
    // need this padding character for the purpose of our tokens, so we omit them.
    token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

    // Generate a SHA-256 hash of the plaintext token string. This will be the value that we store 
    // in the 'hash' field of our database table. Note that the sha256.Sum256() function returns 
    // an *array* of length 32, so to make it easier to work with we convert it ot a slice using 
    // the [:] operator before storing it.
    hash := sha256.Sum256([]byte(token.Plaintext))
    token.Hash = hash[:]

    return token, nil
}

// ValidateTokenPlaintext validates the plaintext token is exactly 26 bytes long.
func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
    v.Check(tokenPlaintext != "", "token", "must be provided")
    v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

// TokenModel struct wraps a database connection pool wrapper.
type TokenModel struct {
    DB *PoolWrapper
}

// New is a shortcut which creates a new Token struct and then inserts the data in the token table.
func (m TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
    token, err := generateToken(userID, ttl, scope)
    if err != nil {
        return nil, err
    }

    err = m.Insert(token)
    return token, err
}

// Insert inserts a new record in the token table.
func (m TokenModel) Insert(token *Token) error {
    query := `INSERT INTO token (hash, user_id, expiry, scope) 
              VALUES ($1, $2, $3, $4)`

    args := []any{token.Hash, token.UserID, token.Expiry, token.Scope}

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    _, err := m.DB.Pool.Exec(ctx, query, args...)

    return err
}

// DeleteAllForUser deletes all tokens for a specific user and scope.
func (m TokenModel) DeleteAllForUser(userID int64, scope string) error {
    query := `DELETE FROM token 
              WHERE user_id = $1 AND scope = $2`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    _, err := m.DB.Pool.Exec(ctx, query, userID, scope)

    return err
}