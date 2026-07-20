package postgresql

import (
	"context"
	"os"
	"sync"
	"time"

	e "github.com/ChatDetectiveORG/shared/errors"
	models "github.com/ChatDetectiveORG/shared/postgresModels"
	// requiredModels "github.com/ChatDetectiveORG/api-gateway/src/infrastructure/postgresql/requiredModels"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

var (
	db   *pg.DB
	once sync.Once
)

func GetDB() *pg.DB {
	once.Do(func() {
		db = pg.Connect(&pg.Options{
			Addr:     os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Database: os.Getenv("POSTGRES_DB"),
			PoolSize: 20, // Устанавливаем разумный размер пула
		})
	})
	return db
}

// Ping verifies the database connection; used by the readiness probe.
func Ping() *e.ErrorInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := GetDB().Ping(ctx); err != nil {
		return e.FromError(err, "postgres ping failed").WithSeverity(e.Notice)
	}
	return e.Nil()
}

func InitPostgresql() *e.ErrorInfo {
	db := GetDB()

	requiredModels := []interface{}{
		// (*requiredModels.LoasUpdates)(nil),
		(*models.Telegramuser)(nil),
		(*models.Payment)(nil),
		(*models.Mirror)(nil),
	}

	for _, model := range requiredModels {
		err := db.Model(model).CreateTable(&orm.CreateTableOptions{
			IfNotExists: true,
		})
		if err != nil {
			return e.FromError(err, "error creating table")
		}
	}

	return nil
}
