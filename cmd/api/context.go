package main

import (
	"context"
	"net/http"

	"github.com/vishaaxl/cheershare/internal/data"
)

// contextKey is a custom type to avoid conflicts with other context keys in the application.
type contextKey string

// userContextKey is the key used to store and retrieve user information from the context.
const userContextKey = contextKey("cheershare.user")

// contextSetUser associates a given user object with the request's context.
//
// Parameters:
// - r: The incoming HTTP request.
// - user: A pointer to a data.User object that represents the authenticated user.
//
// Returns:
// - *http.Request: A new HTTP request with the user data stored in the context.
//
// Usage:
// This function is typically used after authenticating a user to attach the user information
// to the request, enabling downstream handlers to access the user data.
func (app *application) contextSetUser(r *http.Request, user *data.User) *http.Request {
	// Add the user to the context of the incoming request.
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

// contextGetUser retrieves the user object from the request's context.
//
// Parameters:
// - r: The incoming HTTP request.
//
// Returns:
// - *data.User: A pointer to the user object stored in the context.
//
// Panics:
// If the user data is not present in the context or cannot be cast to *data.User,
// the function will panic with the message "missing user context."
//
// Usage:
// This function is used to access the user data attached to the request's context.
// It is generally called in handlers that need user-specific information.
func (app *application) contextGetUser(r *http.Request) *data.User {
	// Retrieve the user from the request context.
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		panic("missing user context")
	}
	return user
}
