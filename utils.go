package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func generateUserId() string {
	id := uuid.New()
	return id.String()
}

// JWT Claims structure
type Claims struct {
	UserID string `json:"userId"`
	jwt.RegisteredClaims
}

func GenerateJWT(userID string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "mangaguesser",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if jwtSecret == nil {
		jwtSecret = []byte("fallback-secret-change-in-production")
		log.Warn("JWT_SECRET not set, using fallback secret")
	}

	return token.SignedString(jwtSecret)
}

func ValidateJWT(tokenString string) (*Claims, error) {
	claims := &Claims{}

	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if jwtSecret == nil {
		jwtSecret = []byte("fallback-secret-change-in-production")
	}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func SetJWTCookie(c *gin.Context, token string) {
	c.SetCookie("mangaguesser_token", token, 86400, "/", "", false, false)
}
