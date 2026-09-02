package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Claims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	OrgID   string `json:"org"`
	Expiry  int64  `json:"exp"`
}

func IssueToken(subject, role, orgID, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		Subject: subject,
		Role:    role,
		OrgID:   orgID,
		Expiry:  time.Now().Add(ttl).Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	sig := sign(body, secret)
	return body + "." + sig, nil
}

func ParseToken(token, secret string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return Claims{}, errors.New("token format invalid")
	}
	if sign(parts[0], secret) != parts[1] {
		return Claims{}, errors.New("token signature invalid")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errors.New("token payload invalid")
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, errors.New("token claims invalid")
	}
	if time.Now().Unix() > claims.Expiry {
		return Claims{}, errors.New("token expired")
	}
	return claims, nil
}

func sign(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
