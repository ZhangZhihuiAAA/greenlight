package main

import (
	"fmt"
	"net/http"
)

// logError() is a generic helper for logging an error message along with
// the current request method and URL as attributes in the log entry.
func (app *application) logError(r *http.Request, err error) {
    var (
        method = r.Method
        uri    = r.URL.RequestURI()
    )

    app.logger.Error(err.Error(), "method", method, "uri", uri)
}

// errorResponse() is a generic helper for sending JSON-formatted error messages to the client 
// with a given status code. Note that we're using the any type for the message parameter, rather 
// than just a string type, as this gives us more flexibility over the values that we can include 
// in the response.
func (app *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
    data := envelope{"error": message}

    err := app.writeJSON(w, status, data, nil)
    if err != nil {
        app.logError(r, err)
        w.WriteHeader(http.StatusInternalServerError)
    }
}

// serverErrorResponse() will be used when our applicatoin encounters an unexpected problem at 
// runtime. It logs the detailed error messages, then uses the errorResponse() helper to send a 
// 500 Internal Server Error status code and JSON response (containing a generic error message) 
// to the client.
func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
    app.logError(r, err)

    message := "the server encountered a problem and could not process your request"
    app.errorResponse(w, r, http.StatusInternalServerError, message)
}

func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
    message := "the requested resource could not be found"
    app.errorResponse(w, r, http.StatusNotFound, message)
}

func (app *application) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
    message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
    app.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
    app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (app *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
    app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}