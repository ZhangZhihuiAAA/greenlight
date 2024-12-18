package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"greenlight.zzh.net/internal/validator"
)

// Movie represents a movie entity.
type Movie struct {
    ID        int64     `json:"id"`                // Unique integer ID for the movie
    CreatedAt time.Time `json:"-"`                 // Timestamp for when the movie is added to our database
    Title     string    `json:"title"`             // Movie title
    Year      int32     `json:"year,omitempty"`    // Movie release year
    Runtime   Runtime   `json:"runtime,omitempty"` // Movie runtime (in minutes)
    Genres    []string  `json:"genres,omitempty"`  // Slice of genres for the movie (romance, comedy, etc.)
    Version   int32     `json:"version"`           // The version number starts at 1 and will be incremented each time the movie information is updated
}

// ValidateMovie validates the fields of movie using validator v.
func ValidateMovie(v *validator.Validator, movie *Movie) {
    v.Check(movie.Title != "", "title", "must be provided")
    v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")

    v.Check(movie.Year != 0, "year", "must be provided")
    v.Check(movie.Year >= 1888, "year", "must be greater than or equal to 1888")
    v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")

    v.Check(movie.Runtime != 0, "runtime", "must be provided")
    v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")

    v.Check(movie.Genres != nil, "genres", "must be provided")
    v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
    v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
    v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}

// MovieModel struct wraps a database connection pool wrapper.
type MovieModel struct {
    DB *PoolWrapper
}

// Insert inserts a new record in the movie table.
func (m MovieModel) Insert(movie *Movie) error {
    query := `INSERT INTO movie (title, year, runtime, genres) 
              VALUES ($1, $2, $3, $4) 
              RETURNING id, created_at, version`

    args := []any{movie.Title, movie.Year, movie.Runtime, movie.Genres}

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    return m.DB.Pool.QueryRow(ctx, query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

// Get returns a specific record from the movie table.
func (m MovieModel) Get(id int64) (*Movie, error) {
    if id < 1 {
        return nil, ErrRecordNotFound
    }

    query := `SELECT id, created_at, title, year, runtime, genres, version 
                FROM movie 
               WHERE id = $1`

    var movie Movie

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    err := m.DB.Pool.QueryRow(ctx, query, id).Scan(
        &movie.ID,
        &movie.CreatedAt,
        &movie.Title,
        &movie.Year,
        &movie.Runtime,
        &movie.Genres,
        &movie.Version,
    )

    if err != nil {
        switch {
        case errors.Is(err, pgx.ErrNoRows):
            return nil, ErrRecordNotFound
        default:
            return nil, err
        }
    }

    return &movie, nil
}

// GetAll returns a slice of movies.
func (m MovieModel) GetAll(title string, genres []string, filter Filter) ([]*Movie, Metadata, error) {
    query := fmt.Sprintf(`
        SELECT count(*) OVER(), id, created_at, title, year, runtime, genres, version 
          FROM movie 
         WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '') 
           AND (genres @> $2 OR $2 = '{}') 
         ORDER BY %s %s, id ASC 
         LIMIT $3 
        OFFSET $4`, filter.sortColumn(), filter.sortDirection())

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    args := []any{title, genres, filter.limit(), filter.offset()}

    rows, err := m.DB.Pool.Query(ctx, query, args...)
    if err != nil {
        return nil, Metadata{}, err
    }
    defer rows.Close()

    totalRecords := 0
    movies := []*Movie{}

    for rows.Next() {
        var movie Movie

        err := rows.Scan(
            &totalRecords,
            &movie.ID,
            &movie.CreatedAt,
            &movie.Title,
            &movie.Year,
            &movie.Runtime,
            &movie.Genres,
            &movie.Version,
        )
        if err != nil {
            return nil, Metadata{}, err
        }

        movies = append(movies, &movie)
    }

    if err = rows.Err(); err != nil {
        return nil, Metadata{}, err
    }

    metadta := calculateMetadata(totalRecords, filter.Page, filter.PageSize)

    return movies, metadta, nil
}

// Update updates a specific record in the movie table.
func (m MovieModel) Update(movie *Movie) error {
    query := `UPDATE movie 
              SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1 
              WHERE id = $5 AND version = $6
              RETURNING version`

    args := []any{
        movie.Title,
        movie.Year,
        movie.Runtime,
        movie.Genres,
        movie.ID,
        movie.Version,  // Add the expected movie version.
    }

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    err := m.DB.Pool.QueryRow(ctx, query, args...).Scan(&movie.Version)
    if err != nil {
        switch {
        case errors.Is(err, pgx.ErrNoRows):
            return ErrEditConflict
        default:
            return err
        }
    }

    return nil
}

// Delete deletes a specific record from the movie table.
func (m MovieModel) Delete(id int64) error {
    if id < 1 {
        return ErrRecordNotFound
    }

    query := `DELETE FROM movie 
              WHERE id = $1`

    ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
    defer cancel()

    result, err := m.DB.Pool.Exec(ctx, query, id)
    if err != nil {
        return err
    }

    if result.RowsAffected() == 0 {
        return ErrRecordNotFound
    }

    return nil
}