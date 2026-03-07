package main

import (
	"ai-outreach-engine/internal/csvreader"
	"ai-outreach-engine/internal/db"
	"ai-outreach-engine/internal/producer"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {

	// --- DB ---
	dbConn := db.ConnectPostgres()
	defer dbConn.Close()

	// --- Rabbit ---
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal("RabbitMQ connection failed:", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Channel open failed:", err)
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		"hr_raw_queue",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": "email_dlq", // route failed messages here
		},
	)
	if err != nil {
		log.Fatal("Queue declare failed:", err)
	}

	// --- Read CSV ---
	rows := csvreader.ReadCSV("data/hr_list.csv")
	log.Println("Rows found:", len(rows))

	// --- Process ---
	producer.Process(rows, dbConn, ch)
}
