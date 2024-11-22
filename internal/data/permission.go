package data

import (
	"context"
	"slices"
	"time"
)

// Permissions stores the permission codes for a single user.
type Permissions []string

// Include checks whether the Permissions slice contains a specific permission code.
func (p Permissions) Include(code string) bool {
    return slices.Contains(p, code)
}

// PermissionModel struct wraps a database connection pool wrapper.
type PermissionModel struct {
    DB *PoolWrapper
}

// GetAllForUser returns all permission codes for a specific user.
func (m PermissionModel) GetAllForUser(userID int64) (Permissions, error) {
    query := `SELECT p.code 
                FROM permission p 
               INNER JOIN user_permission up ON up.permission_id = p.id 
               INNER JOIN users u ON up.user_id = u.id 
               WHERE u.id = $1`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    rows, err := m.DB.Pool.Query(ctx, query, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var permissions Permissions

    for rows.Next() {
        var permission string

        err := rows.Scan(&permission)
        if err != nil {
            return nil, err
        }

        permissions = append(permissions, permission)
    }
    if err = rows.Err(); err != nil {
        return nil, err
    }

    return permissions, nil
}

// AddForUser adds the provided permissions for a specific user.
func (m PermissionModel) AddForUser(userID int64, codes ...string) error {
    query := `INSERT INTO user_permission 
              SELECT $1, id 
                FROM permission 
               WHERE code = ANY($2)`

    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    _, err := m.DB.Pool.Exec(ctx, query, userID, codes)
    return err
}