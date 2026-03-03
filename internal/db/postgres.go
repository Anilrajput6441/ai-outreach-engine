package db

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func ConnectPostgres() *sql.DB {
	conString := "postgres://admin:admin@localhost:5432/outreach?sslmode=disable"

	db, err := sql.Open("postgres", conString)

	if err != nil {
		log.Fatal("Failed to Open DB", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to Connect to DB", err)
	}
	log.Println("Connected to Postgres successfully")
	return db
}
