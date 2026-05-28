package middleware

import (
	"fmt"
	"time"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	DepartmentID string `json:"department_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID, username, role, departmentID string) (string, error) {
	claims := UserClaims{
		UserID:       userID,
		Username:     username,
		Role:         role,
		DepartmentID: departmentID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    config.C.JWT.Issuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.C.JWT.ExpireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.C.JWT.Secret))
}

func ParseToken(tokenStr string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(config.C.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
