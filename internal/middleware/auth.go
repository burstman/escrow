package middleware

import (
	"context"
	"net/http"
	"time"

	"escrow/internal/model"
)

type contextKey string

const UserKey contextKey = "user"

func GetUser(r *http.Request) *model.User {
	u, ok := r.Context().Value(UserKey).(*model.User)
	if !ok {
		return nil
	}
	return u
}

func SetUser(r *http.Request, u *model.User) *http.Request {
	ctx := context.WithValue(r.Context(), UserKey, u)
	return r.WithContext(ctx)
}

type SessionStore interface {
	Get(r *http.Request, name string) (*http.Cookie, error)
	Save(w http.ResponseWriter, r *http.Request, name, value string, expiry time.Time) error
	Clear(w http.ResponseWriter, r *http.Request, name string) error
}

func RequireAuth(store SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := store.Get(r, "session")
			if err != nil || cookie == nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil || user.Role != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
