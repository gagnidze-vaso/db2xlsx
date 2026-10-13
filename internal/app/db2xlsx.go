package app

import (
	"context"
	"database/sql"

	extr "github.com/gagnidze-vaso/db2xlsx/pkg/extractor"
	"github.com/go-sql-driver/mysql"
)

type db2xlsx struct {
	db  *sql.DB
	cfg *mysql.Config
}

func New(db *sql.DB, cfg *mysql.Config) *db2xlsx {
	return &db2xlsx{
		db:  db,
		cfg: cfg,
	}
}

func (c *db2xlsx) Start(ctx context.Context) error {
	extr, err := extr.New(ctx, c.db, c.cfg)
	if err != nil {
		return err
	}

	tables, err := extr.ExtractTables()
	if err != nil {
		return err
	}

	return extr.SaveToXLSX(tables)
}
