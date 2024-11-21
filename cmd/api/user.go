package main

import (
	"errors"
	"net/http"

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

    // Send the welcome email in background.
    app.background(func() {
        err = app.emailSender.Send(user.Email, "user_welcome.html", user)
        if err != nil {
            // Importantly, if there is an error sending the email, we use the app.logger.Error() 
            // helper to manage it, instead of the app.serverErrorResponse() helper like before.
            app.logger.Error(err.Error())
        }
    })

    err = app.writeJSON(w, http.StatusCreated, envelope{"user": user}, nil)
    if err != nil {
        app.serverErrorResponse(w, r, err)
    }
}
