package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/XJIeI5/calculator/internal/storage"
	_ "github.com/mattn/go-sqlite3"
)

func createTables(ctx context.Context, db *sql.DB) error {
	const (
		usersTable = `
		CREATE TABLE IF NOT EXISTS users(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			login TEXT,
			hashedPassword INTEGER NOT NULL
		);`

		expressionsTable = `
		CREATE TABLE IF NOT EXISTS expressions(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hash INTEGER NOT NULL,
			postfixExpression TEXT,
			userId INTEGER NOT NULL,
			status TEXT,
			result TEXT,

			FOREIGN KEY (userId) REFERENCES users (id)
		);`

		timeoutsTable = `
		CREATE TABLE IF NOT EXISTS timeouts(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT,
			value INTEGER NOT NULL
		);`
	)

	if _, err := db.ExecContext(ctx, usersTable); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, expressionsTable); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, timeoutsTable); err != nil {
		return err
	}
	return nil
}

func main() {
	hostPtr := flag.String("host", "http://localhost", "host of server")
	portPtr := flag.Int("port", 8080, "port of server")
	flag.Parse()

	db, err := sql.Open("sqlite3", "store.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err := db.PingContext(context.TODO()); err != nil {
		panic(err)
	}
	if err := createTables(context.TODO(), db); err != nil {
		panic(err)
	}

	go func() {
		fmt.Printf("run storage server at %s:%d\n", *hostPtr, *portPtr)
		s := storage.GetServer(*hostPtr, *portPtr, db)
		s.ListenAndServe()
	}()

	var stopChan = make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-stopChan // wait for SIGINT
	fmt.Println("stop storage server")
}
