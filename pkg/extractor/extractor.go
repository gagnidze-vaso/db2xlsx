package extractor

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/xuri/excelize/v2"
)

type Extractor struct {
	ctx context.Context
	db  *sql.DB
	cfg *mysql.Config
}

type Table struct {
	name    string
	columns []string
	data    []Column
}
type Column map[string]any

func New(ctx context.Context, db *sql.DB, cfg *mysql.Config) (*Extractor, error) {
	return &Extractor{
		ctx: ctx,
		db:  db,
		cfg: cfg,
	}, nil
}

func (extr *Extractor) ExtractTables() (*[]Table, error) {
	tableNames, err := extr.getTableNames()
	if err != nil {
		return nil, err
	}

	var tables []Table
	for _, name := range tableNames {
		table, err := extr.ExtractTable(name)
		if err != nil {
			return nil, err
		}

		tables = append(tables, *table)
	}

	return &tables, nil
}

func (extr *Extractor) ExtractTable(name string) (*Table, error) {
	query := fmt.Sprintf("SELECT * FROM %s", name)
	rows, err := extr.db.QueryContext(extr.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query table %s: %v", name, err)
	}
	defer rows.Close()

	columns, _ := rows.Columns()

	var parsedTable Table
	parsedTable.name = name
	parsedTable.columns = columns

	for rows.Next() {
		columnPtrs := make([]any, len(columns))
		for i := range columns {
			columnPtrs[i] = new(any)
		}

		if err = rows.Scan(columnPtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan table `%s` columns: %v", name, err)
		}

		dest := make(Column, len(columns))
		for i, column := range columns {
			dest[column] = *(columnPtrs[i].(*any))
		}

		parsedTable.data = append(parsedTable.data, dest)
	}

	// Check for errors during row iteration.
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows scan error: %v", err)
	}

	return &parsedTable, nil
}

func (extr *Extractor) getTableNames() ([]string, error) {
	rows, err := extr.db.Query("SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("query tables: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %v", err)
		}
		names = append(names, name)
	}

	// Check for errors during row iteration.
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows scan error: %v", err)
	}

	return names, nil
}

func (extr *Extractor) SaveToXLSX(tables *[]Table) error {
	t := *tables
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	for _, table := range t {
		f.NewSheet(table.name)

		for i, column := range table.columns {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(table.name, cell, column)
		}

		for i, row := range table.data {
			for j, column := range table.columns {
				cell, _ := excelize.CoordinatesToCellName(j+1, i+2)

				f.SetCellValue(table.name, cell, row[column])
			}
		}
	}

	f.DeleteSheet("Sheet1")

	// Save spreadsheet by the given path.
	fileName := fmt.Sprint(extr.cfg.DBName, ".xlsx")
	if err := f.SaveAs(fileName); err != nil {
		return err
	}

	fmt.Printf("Successfully saved to %s", fileName)

	return nil
}
