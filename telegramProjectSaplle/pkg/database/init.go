package database

import (
	// "app/pkg/database/models"
	e "app/pkg/errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

var (
	db   *pg.DB
	once sync.Once
)

// GetDB returns a singleton instance of the database connection
func GetDB() *pg.DB {
	once.Do(func() {
		options := &pg.Options{
			Addr:     os.Getenv("POSTGRES_HOST") + ":" + os.Getenv("POSTGRES_PORT"),
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Database: os.Getenv("POSTGRES_DB"),
			PoolSize: 20, // Устанавливаем разумный размер пула
		}

		// Пытаемся подключиться с повторными попытками
		maxRetries := 30
		retryInterval := 2 * time.Second

		for i := 0; i < maxRetries; i++ {
			db = pg.Connect(options)

			// Проверяем соединение
			if _, err := db.Exec("SELECT 1"); err != nil {
				log.Printf("Failed to connect to database (attempt %d/%d): %v", i+1, maxRetries, err)
				if i < maxRetries-1 {
					time.Sleep(retryInterval)
					continue
				}
				panic(fmt.Sprintf("Could not connect to database after %d attempts: %v", maxRetries, err))
			}

			log.Println("Successfully connected to database")
			break
		}
	})
	return db
}

func InitDb() *e.ErrorInfo {
	db := GetDB()

	models := []interface{}{

	}

	for _, model := range models {
		err := db.Model(model).CreateTable(&orm.CreateTableOptions{
			IfNotExists: true,
		})
		if err != nil {
			return e.FromError(err, "Error creating table").
				WithSeverity(e.Critical).
				WithData(map[string]any{
					"model": model,
				})
		}
	}

	return e.Nil()
}
