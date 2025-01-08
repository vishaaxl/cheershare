package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"time"
)

const (
	ScopeAuthentication = "authentication"
)

type Token struct {
	Plaintext string    `json:"plaintext"`
	Hash      []byte    `json:"-"`
	UserId    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

type TokenModel struct {
	DB *sql.DB
}

/*
Function Parameters:
  - userId (int64): The unique identifier for the user for whom the token is being generated.
  - ttl (time.Duration): The time-to-live for the token, defining its expiry duration.
  - scope (string): The scope or purpose of the token (e.g., authentication, API access).

Return Values:
  - (*Token): A pointer to the generated Token structure, which contains:
  - UserId: The ID of the user associated with the token.
  - Expiry: The exact timestamp when the token will expire.
  - Scope: The defined purpose or scope of the token.
  - Plaintext: The plain, readable token string (used before hashing).
  - Hash: The SHA-256 hash of the plain token for secure storage/verification.
  - (error): An error object if token generation fails at any point.
*/
func generateToken(userId int64, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserId: userId,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	randomBytes := make([]byte, 16)

	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)
	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token, nil

}

func (m TokenModel) Insert(token *Token) error {
	query := `INSERT INTO tokens (hash, user_id, expiry, scope)
	VALUES ($1, $2, $3, $4)`

	args := []interface{}{token.Hash, token.UserId, token.Expiry, token.Scope}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, args...)
	return err
}

func (m TokenModel) DeleteAllForUser(scope string, userID int64) error {
	query := `DELETE FROM tokens  WHERE user_id = $1 AND scope = $2`

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID, scope)
	return err
}

func (m TokenModel) New(userId int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userId, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.Insert(token)
	return token, err
}
