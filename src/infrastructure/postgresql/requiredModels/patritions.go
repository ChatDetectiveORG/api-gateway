package requiredmodelsgo

import "time"

type RegisteredPods struct {
	ID int `pg:"id,pk"`
	CreatedAt time.Time `pg:"created_at,notnull,default:now()"`
	UpdatedAt time.Time `pg:"updated_at,notnull,default:now()"`
	PodName string
	AcceptUpdateType string
	UnhandledUpdates int
	UpdatesAvailable int // summary of updates available for the pod
}
