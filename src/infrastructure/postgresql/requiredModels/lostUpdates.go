package requiredmodelsgo

import "time"

// Модель для сохранения апдейтов, которые упали с ошибкой
// TODO: Имплементировать. Часть данных отсюда перенести в Redis (ResolveTimeout, Retries, MaxRetries), а занесение в постгрес делать только при провале последнего ретраяя
type LoasUpdates struct {
	ID               int       `pg:"id,pk"`
	CreatedAt        time.Time `pg:"created_at,notnull,default:now()"`
	UpdatedAt        time.Time `pg:"updated_at,notnull,default:now()"`
	PodName          string
	ErrorMessage     string
	UpdateContext    string
	ResolveTimeout   int64     `pg:"default:60000"` // ms
	Retries          int       `pg:"default:0"`
	MaxRetries       int       `pg:"default:5"`
}
