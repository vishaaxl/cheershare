package data

import (
	"database/sql"
	"errors"
)

type Models struct {
	User     UserModel
	Creative CreativeModel
	Token    TokenModel
}

var (
	ErrRecordNotFound = errors.New("record not found")
)

func NewModels(db *sql.DB) Models {
	return Models{
		User: UserModel{
			DB: db,
		},
		Token: TokenModel{
			DB: db,
		},
		Creative: CreativeModel{
			DB: db,
		},
	}
}
