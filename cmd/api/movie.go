package main

import (
	"errors"
	"fmt"
	"net/http"

	"greenlight.zzh.net/internal/data"
	"greenlight.zzh.net/internal/validator"
)

func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title   string       `json:"title"`
        Year    int32        `json:"year"`
        Runtime data.Runtime `json:"runtime"`
        Genres  []string     `json:"genres"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    movie := &data.Movie{
        Title:   input.Title,
        Year:    input.Year,
        Runtime: input.Runtime,
        Genres:  input.Genres,
    }

    v := validator.New()

    if data.ValidateMovie(v, movie); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    err = app.models.Movie.Insert(movie)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    // When sending a HTTP response, we want to include a Location header to let the client know
    // at which URL they can find the newly-created resource. We make an empty http.Header map and
    // add a new Location header, interpolating the ID for our new movie in the URL.
    headers := make(http.Header)
    headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))

    err = app.writeJSON(w, http.StatusCreated, envelope{"movie": movie}, headers)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
    id, err := app.readIDParam(r)
    if err != nil {
        app.notFoundResponse(w, r)
        return
    }

    movie, err := app.models.Movie.Get(id)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.notFoundResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
    id, err := app.readIDParam(r)
    if err != nil {
        app.notFoundResponse(w, r)
        return
    }

    movie, err := app.models.Movie.Get(id)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.notFoundResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    var input struct {
        Title   *string       `json:"title"`
        Year    *int32        `json:"year"`
        Runtime *data.Runtime `json:"runtime"`
        Genres  []string      `json:"genres"`
    }

    err = app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    if input.Title != nil {
        movie.Title = *input.Title
    }
    if input.Year != nil {
        movie.Year = *input.Year
    }
    if input.Runtime != nil {
        movie.Runtime = *input.Runtime
    }
    if input.Genres != nil {
        movie.Genres = input.Genres // Note that we don't need to dereference a slice.
    }

    v := validator.New()

    if data.ValidateMovie(v, movie); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    err = app.models.Movie.Update(movie)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrEditConflict):
            app.editConflictResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    err = app.writeJSON(w, http.StatusOK, envelope{"movie": movie}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
    id, err := app.readIDParam(r)
    if err != nil {
        app.notFoundResponse(w, r)
        return
    }

    err = app.models.Movie.Delete(id)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.notFoundResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    err = app.writeJSON(w, http.StatusOK, envelope{"message": "movie successfully deleted"}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Title  string
        Genres []string
        data.Filter
    }

    v := validator.New()

    qs := r.URL.Query()

    input.Title = app.readString(qs, "title", "")
    input.Genres = app.readCSV(qs, "genres", []string{})

    input.Filter.Page = app.readInt(qs, "page", 1, v)
    input.Filter.PageSize = app.readInt(qs, "page_size", 20, v)
    input.Filter.Sort = app.readString(qs, "sort", "id")
    input.Filter.SortSafeList = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

    if data.ValidateFilter(v, input.Filter); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    movies, metadata, err := app.models.Movie.GetAll(input.Title, input.Genres, input.Filter)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    err = app.writeJSON(w, http.StatusOK, envelope{"movies": movies, "metadata": metadata}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
