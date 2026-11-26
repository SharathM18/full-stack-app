package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sharathm18/myapp/internal/domain/shared/apperror"
	"github.com/sharathm18/myapp/internal/domain/shared/utils"
	"github.com/sharathm18/myapp/internal/domain/user"
	"github.com/sharathm18/myapp/internal/middleware"
	"github.com/sharathm18/myapp/internal/server"
	emailTask "github.com/sharathm18/myapp/internal/worker/email"
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
		return nil, "", apperror.Conflict("EMAIL_ALREADY_EXISTS", "an account with this email already exists")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, "", fmt.Errorf("auth service signup: %w", err)
	}

	// hash password with configured bcrypt cost
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.AppServer.Config.Auth.BcryptCost)
	if err != nil {
		return nil, "", fmt.Errorf("auth service signup: %w", err)
	}

	// Atomically create user + initial device session
	var (
		createdUser     *user.UserModel
		rawRefreshToken string
	)

	err = s.AppServer.Database.RunInTx(ctx, func(txCtx context.Context) error {
		var txErr error

		createdUser, txErr = s.UserRepository.CreatePasswordUser(txCtx, name, email, string(hashedBytes))
		if txErr != nil {
			return txErr
		}

		rawRefreshToken, txErr = s.CreateSession(txCtx, createdUser.ID, userAgent, ipAddress)
		if txErr != nil {
			return txErr
		}

		return nil
	})
	if err != nil {
		return nil, "", fmt.Errorf("auth service signup: %w", err)
	}

	// Send a email notification
	task, err := emailTask.NewWelcomeEmailTask(ctx, emailTask.WelcomeEmailPayload{
		To:   email,
		Name: name,
	})
	if err != nil {
		middleware.GetLogger(ctx).Warn().Err(err).Msg("failed to build welcome email task")
	} else if _, err := s.AppServer.TaskQueue.EnqueueTask(ctx, task); err != nil {
		middleware.GetLogger(ctx).Warn().Err(err).Msg("failed to enqueue welcome email")
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
			return nil, "", apperror.Unauthorized("invalid email or password")
		}
		return nil, "", fmt.Errorf("auth service login: %w", err)
	}

	// check account eligibility
	if !existingUser.IsActive {
		return nil, "", apperror.Forbidden("your account has been deactivated")
	}
	if existingUser.PasswordHash == nil {
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(req.Password))
		return nil, "", apperror.Unauthorized("invalid email or password")
	}

	// verify password hash
	if err := bcrypt.CompareHashAndPassword([]byte(*existingUser.PasswordHash), []byte(req.Password)); err != nil {
		return nil, "", apperror.Unauthorized("invalid email or password")
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
