package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/KistametL/WMS/backend/internal/config"
	db "github.com/KistametL/WMS/backend/internal/database/generated"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrUserInactive       = errors.New("user account is inactive")
)

type Service struct {
	queries *db.Queries
	cfg     *config.Config
}

func NewService(pool *pgxpool.Pool, cfg *config.Config) *Service {
	return &Service{
		queries: db.New(pool),
		cfg:     cfg,
	}
}

func (s *Service) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	roles, err := s.queries.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	permissions, err := s.queries.GetUserPermissions(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleNames[i] = r.Name
	}

	permNames := make([]string, len(permissions))
	copy(permNames, permissions)

	accessToken, err := s.generateAccessToken(user.ID.String(), user.Email, roleNames, permNames)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.createRefreshToken(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID.String(),
		Email:        user.Email,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Roles:        roleNames,
	}, nil
}

func (s *Service) RefreshToken(ctx context.Context, req RefreshRequest) (*RefreshResponse, error) {
	tokenHash := hashToken(req.RefreshToken)

	stored, err := s.queries.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, ErrInvalidToken
	}

	user, err := s.queries.GetUserByID(ctx, stored.UserID)
	if err != nil || !user.IsActive {
		return nil, ErrInvalidToken
	}

	if err := s.queries.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}

	roles, _ := s.queries.GetUserRoles(ctx, user.ID)
	permissions, _ := s.queries.GetUserPermissions(ctx, user.ID)

	roleNames := make([]string, len(roles))
	for i, r := range roles {
		roleNames[i] = r.Name
	}
	permNames := make([]string, len(permissions))
	copy(permNames, permissions)

	accessToken, err := s.generateAccessToken(user.ID.String(), user.Email, roleNames, permNames)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.createRefreshToken(ctx, user.ID.String())
	if err != nil {
		return nil, err
	}

	return &RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.queries.RevokeRefreshToken(ctx, hashToken(refreshToken))
}

func (s *Service) generateAccessToken(userID, email string, roles, permissions []string) (string, error) {
	expireHours, _ := strconv.Atoi(s.cfg.JWT.ExpireHours)

	claims := jwt.MapClaims{
		"user_id":     userID,
		"email":       email,
		"roles":       roles,
		"permissions": permissions,
		"exp":         time.Now().Add(time.Duration(expireHours) * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWT.Secret))
}

func (s *Service) createRefreshToken(ctx context.Context, userID string) (string, error) {
	expireDays, _ := strconv.Atoi(s.cfg.JWT.RefreshExpireDays)

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plainToken := base64.URLEncoding.EncodeToString(raw)

	pgUserID, err := parseUUID(userID)
	if err != nil {
		return "", err
	}

	_, err = s.queries.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    pgUserID,
		TokenHash: hashToken(plainToken),
		ExpiresAt: pgTimestamp(time.Now().Add(time.Duration(expireDays) * 24 * time.Hour)),
	})
	if err != nil {
		return "", err
	}

	return plainToken, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
