package data

import "database/sql"

type Models struct {
	User  UserModel
	Token TokenModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		User: UserModel{
			DB: db,
		},
		Token: TokenModel{
			DB: db,
		},
	}
}
