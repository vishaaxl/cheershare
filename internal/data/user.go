package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"
)

type User struct {
	ID          int64     `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	Name        string    `json:"name"`
	PhoneNumber string    `json:"phone_number"`
	Version     int       `json:"version"`
}

type UserModel struct {
	DB *sql.DB
}

var AnonymousUser = &User{}

func (m *User) IsAnonymous() bool {
	return m == AnonymousUser
}

func (m UserModel) Insert(user *User) error {
	query := `
		INSERT INTO users (name, phone_number)
		VALUES ($1, $2)
		RETURNING id, created_at, version
	`

	args := []interface{}{user.Name, user.PhoneNumber}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		return err
	}

	return nil
}

func (m UserModel) GetByPhoneNumber(PhoneNumber string) (*User, error) {
	query := `
		SELECT id, created_at, name, phone_number, version
        FROM users
        WHERE phone_number = $1
	`

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, PhoneNumber).Scan(&user.ID, &user.CreatedAt, &user.Name, &user.PhoneNumber, &user.Version)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (m UserModel) GetForToken(tokenScope, tokenPlainText string) (*User, error) {

	tokenHash := sha256.Sum256([]byte(tokenPlainText))

	query := `SELECT users.id, users.created_at, users.name,  users.phone_number,  users.version
	FROM users
	INNER JOIN tokens
	ON users.id = tokens.user_id
	WHERE tokens.hash = $1 AND tokens.scope = $2 AND tokens.expiry > $3
	`

	args := []interface{}{tokenHash[:], tokenScope, time.Now()}

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Name, &user.PhoneNumber, &user.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err

		}
	}

	return &user, nil
}
