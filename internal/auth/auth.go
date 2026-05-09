package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/store"

	"github.com/golang-jwt/jwt/v5"
)

type Manager struct {
	secret []byte
	store  *store.Store
}

type Claims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func New(secret string, users *store.Store) *Manager {
	return &Manager{
		secret: []byte(secret),
		store:  users,
	}
}

func (m *Manager) IssueToken(user *store.UserIdentity) (string, error) {
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *Manager) AuthenticateToken(ctx context.Context, authHeader string) (*store.UserIdentity, error) {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, store.ErrUnauthorized
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return nil, store.ErrUnauthorized
	}

	if identity, err := m.store.LookupAPIKey(ctx, token); err == nil {
		return identity, nil
	}

	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil {
		return nil, store.ErrUnauthorized
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, store.ErrUnauthorized
	}

	user, err := m.store.GetUser(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrUnauthorized, err)
	}
	return user, nil
}
