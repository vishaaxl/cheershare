package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve(router http.Handler) error {
	/**
	 * Initialize a new HTTP server instance with configurations defined in the application config.
	 * The server includes timeouts for idle, read, and write operations to handle connections gracefully.
	 * The "Handler" chain applies middleware for panic recovery and authentication.
	 */
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.recoverPanic(app.authenticate(router)),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	/**
	 * Create a channel to capture errors that might occur during the shutdown process.
	 * This channel ensures the serve function properly waits for shutdown tasks to complete.
	 */
	shutdownError := make(chan error)

	/**
	 * Start a goroutine to listen for termination signals (e.g., SIGINT, SIGTERM).
	 * Upon receiving a signal, the server initiates a graceful shutdown sequence.
	 */
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		s := <-quit
		app.logger.Printf("Received %v, starting shutdown", s.String())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}

		/**
		 * Wait for any background tasks to complete.
		 * The application's WaitGroup (app.wg) ensures that no tasks are left running.
		 */
		app.logger.Printf("completing background tasks: port %d", app.config.port)
		app.wg.Wait()

		shutdownError <- nil
	}()

	/**
	 * Log the server startup details, including the configured port and environment mode.
	 * The server will start listening for incoming connections.
	 */
	app.logger.Printf("Starting server on port %d in %s mode", app.config.port, app.config.env)
	err := srv.ListenAndServe()

	/**
	 * If the server stops unexpectedly (not due to a shutdown signal), return the error.
	 */
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	/**
	 * Wait for the shutdown process to complete and capture any errors that occur.
	 * If there is an error during shutdown, return it wrapped with additional context.
	 */
	err = <-shutdownError
	if err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	/**
	 * Log that the server stopped successfully and return nil to indicate no errors.
	 */
	app.logger.Println("server stopped :)")

	return nil
}
