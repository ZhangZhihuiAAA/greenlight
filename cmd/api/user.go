package main

import (
	"errors"
	"net/http"
	"time"

	"greenlight.zzh.net/internal/data"
	"greenlight.zzh.net/internal/validator"
)

func (app *application) registerUserHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        Name     string `json:"name"`
        Email    string `json:"email"`
        Password string `json:"password"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    user := &data.User{
        Name:      input.Name,
        Email:     input.Email,
        Activated: false,
    }

    err = user.Password.Set(input.Password)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    v := validator.New()

    if data.ValidateUser(v, user); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    // Insert the user data into the database.
    err = app.models.User.Insert(user)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrDuplicateEmail):
            v.AddError("email", "a user with this email address already exists")
            app.failedValidationResponse(w, r, v.Errors)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // After the user record is created in the database, generate a new activation token
    // for the user.
    token, err := app.models.Token.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    // Send the welcome email in background.
    app.background(func() {
        data := map[string]any{
            "activationToken": token.Plaintext,
            "userID":          user.ID,
        }

        err = app.emailSender.Send(user.Email, "user_welcome.html", data)
        if err != nil {
            app.logger.Error(err.Error())
        }
    })

    err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}

func (app *application) activateUserHandler(w http.ResponseWriter, r *http.Request) {
    var input struct {
        TokenPlaintext string `json:"token"`
    }

    err := app.readJSON(w, r, &input)
    if err != nil {
        app.badRequestResponse(w, r, err)
        return
    }

    v := validator.New()

    if data.ValidateTokenPlaintext(v, input.TokenPlaintext); !v.Valid() {
        app.failedValidationResponse(w, r, v.Errors)
        return
    }

    user, err := app.models.User.GetForToken(data.ScopeActivation, input.TokenPlaintext)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            v.AddError("token", "invalid or expired activation token")
            app.failedValidationResponse(w, r, v.Errors)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // Update the user's activation status.
    user.Activated = true

    // Save the updated user record in database.
    err = app.models.User.Update(user)
    if err != nil {
        switch {
        case errors.Is(err, data.ErrRecordNotFound):
            app.editConflictResponse(w, r)
        default:
            app.serverErrorResponse(w, r, err)
        }
        return
    }

    // If everything went successfully, we delete all activation tokens for the user.
    err = app.models.Token.DeleteAllForUser(user.ID, data.ScopeActivation)
    if err != nil {
        app.serverErrorResponse(w, r, err)
        return
    }

    // Send the updated user details to the client in a JSON response.
    err = app.writeJSON(w, http.StatusOK, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}