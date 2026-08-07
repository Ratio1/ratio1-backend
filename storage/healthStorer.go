package storage

import "context"

func Ping(ctx context.Context) error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	sqlDb, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDb.PingContext(ctx)
}
