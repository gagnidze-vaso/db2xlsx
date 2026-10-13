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

	"github.com/gagnidze-vaso/db2xlsx/internal/app"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var cfg = mysql.NewConfig()
	if err := promptDBConfig(cfg); err != nil {
		return err
	}

	db, err := connectToDB(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db2xlsx := app.New(db, cfg)
	return db2xlsx.Start(ctx)
}

func promptDBConfig(cfg *mysql.Config) error {
	fmt.Print("server: ")
	if _, err := fmt.Scan(&cfg.Addr); err != nil {
		return fmt.Errorf("read server: %w", err)
	}

	fmt.Print("database: ")
	if _, err := fmt.Scan(&cfg.DBName); err != nil {
		return fmt.Errorf("read database: %w", err)
	}

	fmt.Print("username: ")
	if _, err := fmt.Scan(&cfg.User); err != nil {
		return fmt.Errorf("read username: %w", err)
	}

	fmt.Print("password: ")
	pass, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	fmt.Println()
	cfg.Passwd = string(pass)

	cfg.ParseTime = true
	cfg.Net = "tcp"

	return nil
}

func connectToDB(cfg *mysql.Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}

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

	return db, Ping(ctx, db)
}

// Ping the database to verify DSN provided by the user is valid and the
// server accessible. If the ping fails exit the program with an error.
func Ping(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("unable to connect to database: %v", err)
	}

	return nil
}
