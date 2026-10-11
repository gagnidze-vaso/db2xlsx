package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/xuri/excelize/v2"
	"golang.org/x/term"
)

var db *sql.DB

func main() {
	var err error

	cfg := mysql.NewConfig()

	fmt.Print("server: ")
	fmt.Scan(&cfg.Addr)

	fmt.Print("database: ")
	fmt.Scan(&cfg.DBName)

	fmt.Print("username: ")
	fmt.Scan(&cfg.User)

	fmt.Print("password: ")
	pass, err := term.ReadPassword(int(syscall.Stdin))
	cfg.Passwd = string(pass)

	cfg.ParseTime = true
	cfg.Net = "tcp"

	fmt.Println("connection...")
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		// This will not be a connection error, but a DSN parse error or
		// another initialization error.
		log.Fatal("unable to use data source name", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(3)
	db.SetMaxOpenConns(3)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	appSignal := make(chan os.Signal, 3)
	signal.Notify(appSignal, os.Interrupt)

	go func() {
		<-appSignal
		stop()
	}()

	Ping(ctx)

	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		log.Fatalf("query tables: %v", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err = rows.Scan(&tableName); err != nil {
			log.Fatalf("failed to scan table name: %v", err)
		}
		tableNames = append(tableNames, tableName)
	}

	// Check for errors during row iteration.
	if err = rows.Err(); err != nil {
		log.Fatal("rows scan error: ", err)
	}

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	f.SetSheetName("Sheet1", tableNames[0])

	for _, table := range tableNames {
		f.NewSheet(table)

		query := fmt.Sprintf("SELECT * FROM %s", table)
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			log.Fatalf("query table %s: %v", table, err)
			continue
		}
		defer rows.Close()

		var parsedTable []map[string]any

		columns, _ := rows.Columns()

		for rows.Next() {
			columnPtrs := make([]any, len(columns))
			for i := range columns {
				columnPtrs[i] = new(any)
			}

			if err = rows.Scan(columnPtrs...); err != nil {
				log.Fatalf("failed to scan table `%s` columns: %v", table, err)
			}

			dest := make(map[string]any, len(columns))
			for i, column := range columns {
				dest[column] = *(columnPtrs[i].(*any))
			}

			parsedTable = append(parsedTable, dest)
		}

		// Check for errors during row iteration.
		if err = rows.Err(); err != nil {
			log.Fatal("rows scan error: ", err)
			return
		}

		for i, column := range columns {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(table, cell, column)
		}

		for i, row := range parsedTable {
			for j, column := range columns {
				cell, _ := excelize.CoordinatesToCellName(j+1, i+2)

				f.SetCellValue(table, cell, row[column])
			}
		}
	}

	// Save spreadsheet by the given path.
	fileName := fmt.Sprint(cfg.DBName, ".xlsx")
	if err = f.SaveAs(fileName); err != nil {
		fmt.Println(err)
	}
}

// Ping the database to verify DSN provided by the user is valid and the
// server accessible. If the ping fails exit the program with an error.
func Ping(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
}
