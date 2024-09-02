package server

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"ethereum-fetcher/internal/store"

	"github.com/golang-jwt/jwt/v4"
)

const (
	userIDKey string = "LimeUserID"
)

type AuthBearerMiddleware struct {
	jwtSecret string
	next      http.HandlerFunc
	optional  bool
}

// NewAuthBearerMiddleware handles Authentication Bearer for the user-protected endpoints
func NewAuthBearerMiddleware(jwtSecret string, next http.HandlerFunc, optional bool) *AuthBearerMiddleware {
	return &AuthBearerMiddleware{jwtSecret: jwtSecret, next: next, optional: optional}
}

// Authenticate will verify the jwt token and set userIDKey to the provided value or 0
// in case the "optional" flag is true and the token is missing
func (wab *AuthBearerMiddleware) Authenticate(w http.ResponseWriter, r *http.Request) {
	// extract the token from the header
	authHeader := r.Header.Get("Authorization")

	ctx := context.WithValue(r.Context(), userIDKey, store.NonAuthenticatedUser)

	// continue with request processing when no token is provided and optional flag is true
	if wab.optional && len(authHeader) == 0 {
		wab.next(w, r.WithContext(ctx))
		return
	}

	trimRe := regexp.MustCompile(`^(?i)Bearer\s`)
	tokenString := trimRe.ReplaceAllString(authHeader, "")

	// verify the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(wab.jwtSecret), nil
	})

	if err != nil {
		writeUnauthorizedError(w)
		return
	}

	// verify the claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid || claims.Valid() != nil {
		writeUnauthorizedError(w)
		return
	}

	// extract the "sub" claim, which should contain the user id
	sub, ok := claims["sub"].(float64)
	if !ok {
		writeUnauthorizedError(w)
		return
	}

	// attach user ID to the request context
	ctx = context.WithValue(r.Context(), userIDKey, int(sub))

	// authorized to continue with request processing with attached userID
	wab.next(w, r.WithContext(ctx))
}
