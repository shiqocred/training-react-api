package utils

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Claims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type BearerKey struct {
	ID         string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
	ExpiredAt  time.Time
}

func EnsureBearerKey(ctx context.Context, pool *pgxpool.Pool) (BearerKey, error) {
	var key BearerKey
	var publicKeyEncoded, privateKeyEncoded string
	err := pool.QueryRow(ctx, `SELECT id, x, secret_key, expired_at FROM jwks WHERE kty='OKP' AND crv='Ed25519' AND expired_at > now() ORDER BY created_at DESC LIMIT 1`).Scan(&key.ID, &publicKeyEncoded, &privateKeyEncoded, &key.ExpiredAt)
	if err == nil {
		publicKey, err := base64.RawURLEncoding.DecodeString(publicKeyEncoded)
		if err != nil {
			return BearerKey{}, fmt.Errorf("public key jwks tidak valid: %w", err)
		}
		privateKey, err := base64.RawURLEncoding.DecodeString(privateKeyEncoded)
		if err != nil {
			return BearerKey{}, fmt.Errorf("private key jwks tidak valid: %w", err)
		}
		if len(publicKey) != ed25519.PublicKeySize || len(privateKey) != ed25519.PrivateKeySize {
			return BearerKey{}, errors.New("ukuran key jwks tidak valid")
		}
		key.PublicKey = ed25519.PublicKey(publicKey)
		key.PrivateKey = ed25519.PrivateKey(privateKey)
		return key, nil
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return BearerKey{}, err
	}
	id, err := NewCUID2()
	if err != nil {
		return BearerKey{}, err
	}
	expiredAt := time.Now().AddDate(1, 0, 0)
	_, err = pool.Exec(ctx, `INSERT INTO jwks (id, kty, crv, x, secret_key, expired_at) VALUES ($1, 'OKP', 'Ed25519', $2, $3, $4)`, id, base64.RawURLEncoding.EncodeToString(publicKey), base64.RawURLEncoding.EncodeToString(privateKey), expiredAt)
	if err != nil {
		return BearerKey{}, err
	}
	return BearerKey{ID: id, PublicKey: publicKey, PrivateKey: privateKey, ExpiredAt: expiredAt}, nil
}

func CreateBearerToken(userID, role, issuer string, privateKey ed25519.PrivateKey, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(privateKey)
}

func ParseBearerToken(tokenString, issuer string, publicKey ed25519.PublicKey) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodEdDSA {
			return nil, errors.New("metode token tidak valid")
		}
		return publicKey, nil
	}, jwt.WithIssuer(issuer))
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("token tidak valid")
	}
	return claims, nil
}
