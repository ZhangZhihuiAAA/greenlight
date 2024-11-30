package main

import (
	"errors"
	"expvar"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tomasen/realip"
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
                if time.Since(client.lastSeen) > 3*time.Minute {
                    delete(clients, ip)
                }
            }

            mu.Unlock()
        }
    }()

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if app.config.limiter.Enabled {
            // Use the realip.FromRequest() function to ge the client's real IP address.
            ip := realip.FromRequest(r)

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
    fn := func(w http.ResponseWriter, r *http.Request) {
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

func (app *application) enableCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Add the "Vary: Origin" header.
        w.Header().Add("Vary", "Origin")

        // Add the "Vary: Access-Control-Request-Method" header.
        w.Header().Add("Vary", "Access-Control-Request-Method")

        origin := r.Header.Get("Origin")

        // Only run this if there's an Origin request header present.
        if origin != "" {
            for _, o := range app.config.cors.trustedOrigins {
                if origin == o {
                    w.Header().Set("Access-Control-Allow-Origin", origin)

                    // Check if the request has the HTTP method OPTIONS and contains the
                    // "Access-Control-Request-Method" header. If it does, we treat it as a
                    // preflight request.
                    if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
                        w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
                        w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

                        w.WriteHeader(http.StatusOK)
                        return
                    }

                    break
                }
            }
        }

        next.ServeHTTP(w, r)
    })
}

// The metricsResponseWriter type wraps an existing http.ResponseWriter and also
// contains a field for recording the response status code, and a boolen flag
// to indicate whether the response headers have already been written.
type metricsResponseWriter struct {
    wrapped       http.ResponseWriter
    statusCode    int
    headerWritten bool
}

func newMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
    return &metricsResponseWriter{
        wrapped:    w,
        statusCode: http.StatusOK,
    }
}

// Header is a simple 'pass through' to the Header() method of the wrapped
// http.ResponseWriter.
func (mrw *metricsResponseWriter) Header() http.Header {
    return mrw.wrapped.Header()
}

// WriteHeader does a 'pass through' to the WriteHeader() method of the wrapped
// http.ResponseWriter. But after this returns, we also record the response status
// code (if it hasn't already been recorded) and set the headerWritten field to
// true to indicate that the HTTP response headers have now been written.
func (mrw *metricsResponseWriter) WriteHeader(statusCode int) {
    mrw.wrapped.WriteHeader(statusCode)

    if !mrw.headerWritten {
        mrw.statusCode = statusCode
        mrw.headerWritten = true
    }
}

// Write does a 'pass through' to the Write() method of the wrapped http.ResponseWriter.
// Calling this will automatically write any response headers, so we set the
// headerWritten field to true.
func (mrw *metricsResponseWriter) Write(b []byte) (int, error) {
    mrw.headerWritten = true
    return mrw.wrapped.Write(b)
}

// Unwrap returns the existing wrapped http.ResponseWriter.
func (mrw *metricsResponseWriter) Unwrap() http.ResponseWriter {
    return mrw.wrapped
}

func (app *application) metrics(next http.Handler) http.Handler {
    var (
        totalRequestsReceived           = expvar.NewInt("total_requests_received")
        totalResponsesSent              = expvar.NewInt("total_responses_sent")
        totalProcessingTimeMicroseconds = expvar.NewInt("total_processing_time_μs")
        totalResponsesSentByStatus      = expvar.NewMap("total_responses_sent_by_status")
    )

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()

        totalRequestsReceived.Add(1)

        mrw := newMetricsResponseWriter(w)

        next.ServeHTTP(mrw, r)

        totalResponsesSent.Add(1)

        totalResponsesSentByStatus.Add(strconv.Itoa(mrw.statusCode), 1)

        duration := time.Since(start).Microseconds()
        totalProcessingTimeMicroseconds.Add(duration)
    })
}
