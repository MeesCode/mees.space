package auth

import (
	"context"
	"net/http"
	"strings"

	"mees.space/internal/httputil"
)

type contextKey string

const UserContextKey contextKey = "user"

type UserInfo struct {
	ID       int
	Username string
}

func RequireAuth(jwtSvc *JWTService, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httputil.JSONError(w, "missing authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			httputil.JSONError(w, "invalid authorization format", http.StatusUnauthorized)
			return
		}

		claims, err := jwtSvc.ValidateAccessToken(parts[1])
		if err != nil {
			httputil.JSONError(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			httputil.JSONError(w, "invalid token claims", http.StatusUnauthorized)
			return
		}
		username, _ := claims["username"].(string)

		ctx := context.WithValue(r.Context(), UserContextKey, &UserInfo{
			ID:       int(userIDFloat),
			Username: username,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUser(ctx context.Context) *UserInfo {
	u, _ := ctx.Value(UserContextKey).(*UserInfo)
	return u
}

// OptionalAuth is middleware that attaches user info to the context if a valid
// token is present, but does not reject unauthenticated requests.
func OptionalAuth(jwtSvc *JWTService, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				if claims, err := jwtSvc.ValidateAccessToken(parts[1]); err == nil {
					userIDFloat, ok := claims["user_id"].(float64)
					if ok {
						username, _ := claims["username"].(string)
						ctx := context.WithValue(r.Context(), UserContextKey, &UserInfo{
							ID:       int(userIDFloat),
							Username: username,
						})
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
