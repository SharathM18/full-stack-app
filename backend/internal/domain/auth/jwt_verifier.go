package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sharathm18/myapp/internal/middleware"
	"github.com/sharathm18/myapp/internal/server"
)

// CustomClaims defines the exact JSON fields inside your JWT token
type CustomClaims struct {
	UserRole string `json:"userRole"`
	jwt.RegisteredClaims
}

type JWTVerifier struct {
	AppServer *server.APP
}

func NewJWTVerifier(appServer *server.APP) *JWTVerifier {
	return &JWTVerifier{
		AppServer: appServer,
	}
}

func (v *JWTVerifier) VerifyToken(ctx context.Context, tokenString string) (*middleware.AuthClaims, error) {
	claims := &CustomClaims{}
	secretKey := []byte(v.AppServer.Config.Auth.SecretKey)
	issuer := v.AppServer.Config.Primary.ServiceName

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			return secretKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("verify jwt: %w", err)
	}

	if claims.Subject == "" {
		return nil, errors.New("verify jwt: missing subject (user_id)")
	}

	userID, err := uuid.Parse(claims.RegisteredClaims.Subject)
	if err != nil {
		return nil, fmt.Errorf("verify jwt: invalid subject (user_id): %w", err)
	}

	return &middleware.AuthClaims{
		UserID:   userID,
		UserRole: claims.UserRole,
	}, nil
}

func (v *JWTVerifier) GenerateAccessToken(userID uuid.UUID, userRole string) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserRole: userRole,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    v.AppServer.Config.Primary.ServiceName,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(v.AppServer.Config.Auth.AccessTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(v.AppServer.Config.Auth.SecretKey))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}
