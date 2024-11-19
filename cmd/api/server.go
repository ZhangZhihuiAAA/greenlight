package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
    srv := &http.Server{
        Addr:         fmt.Sprintf(":%d", app.config.port),
        Handler:      app.routes(),
        IdleTimeout:  time.Minute,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 10 * time.Second,
        ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
    }

    // The shutdownError channel is used to receive any errors returned by the 
    // graceful Shutdown() function.
    shutdownError := make(chan error)

    // Start a background goroutine to catch signals.
    go func() {
        quit := make(chan os.Signal, 1)

        // Use signal.Notify() to listen for incoming SIGINT and SIGTERM signals and 
        // relay them to the quit channel.
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

        // Read the signal from the quit channel. This code will block until a signal is received.
        s := <- quit

        app.logger.Info("shutting down server", "signal", s.String())

        ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
        defer cancel()

        shutdownError <- srv.Shutdown(ctx)
    }()

    app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

    err := srv.ListenAndServe()
    if !errors.Is(err, http.ErrServerClosed) {
        return err
    }

    err = <-shutdownError
    if err != nil {
        return err
    }

    app.logger.Info("stopped server", "addr", srv.Addr)

    return nil
}