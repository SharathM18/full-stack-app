package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v5"
	"github.com/sharathm18/myapp/internal/domain/shared/utils"
	"github.com/sharathm18/myapp/internal/domain/user"
	"github.com/sharathm18/myapp/internal/server"
	"golang.org/x/crypto/bcrypt"
)

// UserRepository defines the contract `auth` expects from the `user` domain
type UserRepository interface {
	CreatePasswordUser(ctx context.Context, name, email, passwordHash string) (*user.UserModel, error)
	GetByEmail(ctx context.Context, email string) (*user.UserModel, error)
	GetByID(ctx context.Context, id uuid.UUID) (*user.UserModel, error)
}

// Package-level dummy hash
var dummyBcryptHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

type AuthService struct {
	AppServer      *server.APP
	AuthRepository *AuthRepository
	UserRepository UserRepository
	JWTVerifier    *JWTVerifier
}

func NewAuthService(appServer *server.APP, authRepository *AuthRepository, userRepository UserRepository, jwtVerifier *JWTVerifier) *AuthService {
	return &AuthService{
		AppServer:      appServer,
		AuthRepository: authRepository,
		UserRepository: userRepository,
		JWTVerifier:    jwtVerifier,
	}
}

// main service functions

func (s *AuthService) Signup(ctx context.Context, req SignupRequest, userAgent, ipAddress string) (*AuthResponse, string, error) {
	// normalize inputs
	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// pre-check if email already exists before incurring bcrypt CPU cost
	_, err := s.UserRepository.GetByEmail(ctx, email)
	if err == nil {
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(req.Password))
		return nil, "", echo.NewHTTPError(http.StatusConflict, "an account with this email already exists")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", fmt.Errorf("auth service signup: %w", err)
	}

	// hash password with configured bcrypt cost
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.AppServer.Config.Auth.BcryptCost)
	if err != nil {
		return nil, "", fmt.Errorf("auth service signup: %w", err)
	}

	// atomically create user + initial device session
	var (
		createdUser     *user.UserModel
		rawRefreshToken string
	)

	err = s.AppServer.Database.RunInTx(ctx, func(txCtx context.Context) error {
		var txErr error

		// query 1: runs on the locked transaction connection
		createdUser, txErr = s.UserRepository.CreatePasswordUser(txCtx, name, email, string(hashedBytes))
		if txErr != nil {
			return fmt.Errorf("auth service signup: %w", txErr)
		}

		// query 2: runs on the EXACT SAME locked transaction connection
		rawRefreshToken, txErr = s.CreateSession(txCtx, createdUser.ID, userAgent, ipAddress)
		if txErr != nil {
			return fmt.Errorf("auth service signup: %w", txErr)
		}

		return nil // returning nil triggers COMMIT on that connection
	})
	if err != nil {
		return nil, "", fmt.Errorf("auth service signup: %w", err)
	}

	return s.buildAuthResponse(createdUser, rawRefreshToken)
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest, userAgent, ipAddress string) (*AuthResponse, string, error) {
	// normalize email
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// fetch user by email
	existingUser, err := s.UserRepository.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(req.Password))
			return nil, "", echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
		}
		return nil, "", fmt.Errorf("auth service login: %w", err)
	}

	// check account eligibility
	if !existingUser.IsActive {
		return nil, "", echo.NewHTTPError(http.StatusForbidden, "your account has been deactivated")
	}
	if existingUser.PasswordHash == nil {
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(req.Password))
		return nil, "", echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
	}

	// verify password hash
	if err := bcrypt.CompareHashAndPassword([]byte(*existingUser.PasswordHash), []byte(req.Password)); err != nil {
		return nil, "", echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
	}

	// create new device session and mint Access JWT
	rawRefreshToken, err := s.CreateSession(ctx, existingUser.ID, userAgent, ipAddress)
	if err != nil {
		return nil, "", fmt.Errorf("auth service login: %w", err)
	}

	return s.buildAuthResponse(existingUser, rawRefreshToken)
}

// helper functions

func (s *AuthService) CreateSession(ctx context.Context, userID uuid.UUID, userAgent, ipAddress string) (string, error) {
	rawRefreshToken, err := generateOpaqueToken(32)
	if err != nil {
		return "", fmt.Errorf("auth service generate refresh token: %w", err)
	}

	params := &CreateSessionParams{
		UserID:           userID,
		RefreshTokenHash: hashOpaqueToken(rawRefreshToken),
		UserAgent:        utils.NilIfEmpty(userAgent),
		IPAddress:        utils.NilIfEmpty(ipAddress),
		ExpiresAt:        time.Now().Add(s.AppServer.Config.Auth.RefreshTokenTTL),
	}
	_, err = s.AuthRepository.CreateSession(ctx, params)
	if err != nil {
		return "", fmt.Errorf("auth service create session: %w", err)
	}

	return rawRefreshToken, nil
}

func (s *AuthService) buildAuthResponse(u *user.UserModel, rawRefreshToken string) (*AuthResponse, string, error) {
	accessToken, err := s.JWTVerifier.GenerateAccessToken(u.ID, u.Role)
	if err != nil {
		return nil, "", fmt.Errorf("auth service generate access token: %w", err)
	}

	res := &AuthResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.AppServer.Config.Auth.AccessTokenTTL.Seconds()),
		User: UserDTO{
			ID:          u.ID,
			Name:        u.Name,
			Email:       u.Email,
			PhoneNumber: u.PhoneNumber,
			Role:        u.Role,
		},
	}

	return res, rawRefreshToken, nil
}

// utils functions

func generateOpaqueToken(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth service generate opaque token: %w", err)
	}
	return "rt_" + base64.RawURLEncoding.EncodeToString(b), nil
}

func hashOpaqueToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
