package main

import (
	"context"
	"net/http"

	"greenlight.zzh.net/internal/data"
)

type glContextKey string

// Convert the string "user" to a glContextKey type and assign it to the userContextKey constant.
// We'll use this constant as the key for getting and setting user information in the request 
// context.
const userContextKey = glContextKey("user")

// contextSetUser returns a new copy of the request with the provided User struct added to its 
// embedded context. 
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
    ctx := context.WithValue(r.Context(), userContextKey, user)
    return r.WithContext(ctx)
}

// contextGetUser retrieves the User struct from the request context. The only time that we'll use 
// this helper is when we logically expect there to be User struct value in the context, and if it 
// doesn't exist it will firmly be an 'unexpected' error. It's OK to panic in those circumstances.
func (app *application) contextGetUser(r *http.Request) *data.User {
    user, ok := r.Context().Value(userContextKey).(*data.User)
    if !ok {
        panic("missing user value in request context")
    }

    return user
}