package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
    srv := &http.Server{
        Addr:         app.config.serverAddress,
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

        // Call Shutdown() on the server like before, but now we only send on the shutdownError 
        // channel if it returns an error.
        err := srv.Shutdown(ctx)
        if err != nil {
            shutdownError <- err
        }

        // Log a message to say that we're waiting for any background goroutines to complete 
        // their tasks.
        app.logger.Info("waiting for background tasks to complete", "addr", srv.Addr)

        // Call Wait() to block until the WaitGroup counter is zero -- essentially blocking until 
        // the background goroutines have finished. Then we return nil on the shutdownError 
        // channel, to indicate that the shutdown completed without any issues.
        app.wg.Wait()
        shutdownError <- nil
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