package main

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
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
