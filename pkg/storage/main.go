package storage

import (
	"context"
)

type Client interface {
	ListZones(context.Context, ...string) ([]Zone, error)
}

type Zone interface {
	ListRecords(context.Context, ...string) ([]Record, error)
}

type Record interface {
	GetName() string
	GetType() string
	GetTTL() int64
	GetData() []string
}
