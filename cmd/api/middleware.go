package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"greenlight.zzh.net/internal/data"
	"greenlight.zzh.net/internal/validator"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Create a deferred function which will always be run in the event of a panic
        // as Go unwinds the stack.
        defer func() {
            // Use the builtin recover function to check if there has been a panic or not.
            if err := recover(); err != nil {
                // If there was a panic, set a "Connection: close" header on the response.
                // This acts as a trigger to make Go's HTTP server automatically close the
                // current connection after a response has been sent.
                w.Header().Set("Connection", "close")
                // The value returned by recover() has the type any, so we use fmt.Errorf() to
                // normalize it into an error and call our serverErrorResponse() helper.
                app.serverErrorResponse(w, r, fmt.Errorf("%s", err))
            }
        }()

        next.ServeHTTP(w, r)
    })
}

func (app *application) rateLimit(next http.Handler) http.Handler {
    type client struct {
        limiter  *rate.Limiter
        lastSeen time.Time
    }

    var (
        mu      sync.Mutex
        clients = make(map[string]*client)
    )

    // Launch a background goroutine which removes old entries from the clients map 
    // once every minute.
    go func() {
        for {
            time.Sleep(time.Minute)

            mu.Lock()
            
            for ip, client := range clients {
                if time.Since(client.lastSeen) > 3 * time.Minute {
                    delete(clients, ip)
                }
            }

            mu.Unlock()
        }
    }()

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if app.config.limiter.Enabled {
            ip, _, err := net.SplitHostPort(r.RemoteAddr)
            if err != nil {
                app.serverErrorResponse(w, r, err)
                return
            }

            mu.Lock()

            if _, found := clients[ip]; !found {
                clients[ip] = &client{
                    limiter: rate.NewLimiter(rate.Limit(app.config.limiter.Rps), app.config.limiter.Burst),
                }
            }

            clients[ip].lastSeen = time.Now()
    
            if !clients[ip].limiter.Allow() {
                mu.Unlock()
                app.rateLimitExceededResponse(w, r)
                return
            }

            mu.Unlock()
        }

        next.ServeHTTP(w, r)
    })
}

func (app *application) authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Add the "Vary: Authorization" header to the response. This indicates to any caches that 
        // the response may vary based on the value of the Authorization header in the request.
        w.Header().Add("Vary", "Authorization")

        // Retrieve the value of the Authorization header from the request. 
        // This will return the empty string "" if there is no such header.
        authorizationHeader := r.Header.Get("Authorization")

        // If there is no Authorization header found, add the AnonymousUser to the request 
        // context. Then we call the next handler in the chain and return without executing 
        // any of the code below.
        if authorizationHeader == "" {
            r = app.contextSetUser(r, data.AnonymousUser)
            next.ServeHTTP(w, r)
            return
        }

        // Otherwise, try to split the Authorization header into its constituent parts. If the 
        // header isn't in the expected format, we return a 401 Unauthorized response.
        headerParts := strings.Split(authorizationHeader, " ")
        if len(headerParts) != 2 || headerParts[0] != "Bearer" {
            app.invalidAuthenticationTokenResponse(w, r)
            return
        }

        token := headerParts[1]

        v := validator.New()

        if data.ValidateTokenPlaintext(v, token); !v.Valid() {
            app.invalidAuthenticationTokenResponse(w, r)
            return
        }

        user, err := app.models.User.GetForToken(data.ScopeAuthentication, token)
        if err != nil {
            switch {
            case errors.Is(err, data.ErrRecordNotFound):
                app.invalidAuthenticationTokenResponse(w, r)
            default:
                app.serverErrorResponse(w, r, err)
            }
            return
        }

        r = app.contextSetUser(r, user)

        next.ServeHTTP(w, r)
    })
}

func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := app.contextGetUser(r)

        if user.IsAnonymous() {
            app.authenticationRequiredResponse(w, r)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
    // Rather than returning this http.HandlerFunc we assign it to the variable fn.
    fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        user := app.contextGetUser(r)

        if !user.Activated {
            app.inactiveAccountResponse(w, r)
            return
        }

        next.ServeHTTP(w, r)
    })

    return app.requireAuthenticatedUser(fn)
}

func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
    fn := func(w http.ResponseWriter, r *http.Request)  {
        user := app.contextGetUser(r)

        permissions, err := app.models.Permission.GetAllForUser(user.ID)
        if err != nil {
            app.serverErrorResponse(w, r, err)
            return
        }

        if !permissions.Include(code) {
            app.notPermittedResponse(w, r)
            return
        }

        next.ServeHTTP(w, r)
    }

    return app.requireActivatedUser(fn)
}