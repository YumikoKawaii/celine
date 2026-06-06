package mneme

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewDB opens a GORM connection to Postgres and caps the connection pool.
// Returns a *gorm.DB ready to use; call CloseDB to release it on shutdown.
func NewDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Suppress GORM's default query logging; add a real logger if needed.
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// Cap pool size — §resource: tiny server, ~300–400 MB target.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)

	return db, nil
}

// CloseDB releases the underlying sql.DB connection pool.
func CloseDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
