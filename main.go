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

	var tableList []string

	for rows.Next() {
		var tableName string
		rows.Scan(&tableName)

		tableList = append(tableList, tableName)
	}

	// Check for errors during row iteration.
	if err = rows.Err(); err != nil {
		log.Fatal("rows scan error: ", err)
		return
	}

	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	f.SetSheetName("Sheet1", tableList[0])
	for _, table := range tableList {
		f.NewSheet(table)

		query := fmt.Sprintf("SELECT * FROM %s", table)
		rows, err = db.Query(query)
		if err != nil {
			panic(err)
		}

		parsedTable := make([]map[string]any, 0)

		columns, _ := rows.Columns()

		for rows.Next() {
			raw := make([]any, len(columns))
			for i := range columns {
				raw[i] = new(any)
			}

			rows.Scan(raw...)

			strColumn := make(map[string]any, len(columns))
			for i, column := range columns {
				strColumn[column] = *raw[i].(*any)
			}

			parsedTable = append(parsedTable, strColumn)
		}

		// Check for errors during row iteration.
		if err = rows.Err(); err != nil {
			log.Fatal("rows scan error: ", err)
			return
		}

		for i, column := range columns {
			cell, err := excelize.CoordinatesToCellName(i+1, 1)
			if err != nil {
				log.Fatal(err)
			}

			f.SetCellValue(table, cell, column)
		}

		for i := 0; i < len(parsedTable); i++ {
			for j, column := range columns {
				cell, err := excelize.CoordinatesToCellName(j+1, i+2)
				if err != nil {
					log.Fatal(err)
				}

				f.SetCellValue(table, cell, parsedTable[i][column])
			}
		}
	}

	// Save spreadsheet by the given path.
	fileName := fmt.Sprint(cfg.DBName, ".xlsx")
	if err := f.SaveAs(fileName); err != nil {
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
