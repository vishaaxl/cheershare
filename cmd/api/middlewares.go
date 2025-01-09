package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vishaaxl/cheershare/internal/data"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Connection", "close")
				app.errorResponse(w, http.StatusInternalServerError, "Failed to recover")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add the "Vary: Authorization" header to the response. This indicates to any
		// caches that the response may vary based on the value of the Authorization
		// header in the request.
		w.Header().Add("Vary", "Authorization")

		// Retrieve the value of the Authorization header from the request. This will
		// return the empty string "" if there is no such header found.
		authorizationHeader := r.Header.Get("Authorization")
		// If there is no Authorization header found, use the contextSetUser() helper
		// that we just made to add the AnonymousUser to the request context. Then we
		// call the next handler in the chain and return without executing any of the
		// code below.
		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.errorResponse(w, http.StatusUnauthorized, "Invalid authorization header")
			return
		}

		token := headerParts[1]
		if len(token) != 26 {
			app.errorResponse(w, http.StatusUnauthorized, "Invalid authorization header")
			return
		}
		// validate the token for length and required params

		user, err := app.models.User.GetForToken(data.ScopeAuthentication, token)

		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.errorResponse(w, http.StatusUnauthorized, "Invalid authorization header")
			default:
				app.errorResponse(w, http.StatusInternalServerError, "Can't find user for specified token")
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
			app.errorResponse(w, http.StatusUnauthorized, "Unauthorized")
			return
		}

		next.ServeHTTP(w, r)
	})
}
