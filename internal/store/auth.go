package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/TotallyLegitimateOrg/Mangashelf/internal/apikeys"
	"github.com/TotallyLegitimateOrg/Mangashelf/internal/db/gen"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (s *Store) NeedsSetup(ctx context.Context) (bool, error) {
	count, err := s.queries.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (s *Store) CreateInitialUser(ctx context.Context, username string, password string) (*UserIdentity, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 6 {
		return nil, fmt.Errorf("%w: username and password are required", ErrValidation)
	}

	count, err := s.queries.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSetupAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}

	user := &UserIdentity{
		ID:       uuid.NewString(),
		Username: username,
	}
	if err := s.queries.CreateUser(ctx, gen.CreateUserParams{
		ID:           user.ID,
		Username:     user.Username,
		PasswordHash: string(hash),
		CreatedAt:    nowUnix(),
	}); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Store) Login(ctx context.Context, username string, password string) (*UserIdentity, error) {
	user, err := s.queries.GetUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrUnauthorized
	}
	return &UserIdentity{ID: user.ID, Username: user.Username}, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (*UserIdentity, error) {
	user, err := s.queries.GetUserByID(ctx, id)
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &UserIdentity{ID: user.ID, Username: user.Username}, nil
}

func (s *Store) LookupAPIKey(ctx context.Context, apiKey string) (*UserIdentity, error) {
	key, err := s.queries.GetAPIKeyByHash(ctx, apikeys.Hash(strings.TrimSpace(apiKey)))
	if err != nil {
		if isNotFoundError(err) {
			return nil, ErrUnauthorized
		}
		return nil, err
	}
	return &UserIdentity{ID: key.UserID, Username: key.Username}, nil
}

func (s *Store) CreateAPIKey(ctx context.Context, userID string, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	rawKey, keyPrefix, keyHash, err := apikeys.Generate()
	if err != nil {
		return "", err
	}
	if err := s.queries.CreateAPIKey(ctx, gen.CreateAPIKeyParams{
		ID:        uuid.NewString(),
		UserID:    userID,
		Name:      name,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		CreatedAt: nowUnix(),
	}); err != nil {
		return "", err
	}
	return rawKey, nil
}

func (s *Store) ListAPIKeys(ctx context.Context, userID string) ([]map[string]any, error) {
	keys, err := s.queries.ListAPIKeysByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		result = append(result, map[string]any{
			"id":        key.ID,
			"name":      key.Name,
			"prefix":    key.KeyPrefix,
			"createdAt": key.CreatedAt,
		})
	}
	return result, nil
}

func (s *Store) DeleteAPIKey(ctx context.Context, userID string, apiKey string) error {
	return s.queries.DeleteAPIKey(ctx, gen.DeleteAPIKeyParams{
		ID:     apiKey,
		UserID: userID,
	})
}
