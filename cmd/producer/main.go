package main

import (
	"ai-outreach-engine/internal/db"
	"ai-outreach-engine/internal/models"
	"encoding/json"
	"log"

	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {

	//-------------------------------------------------------
	// --- Postgres DB Connection ---
	//-------------------------------------------------------

	dbConn := db.ConnectPostgres()
	defer dbConn.Close()

	//-------------------------------------------------------
	// --- RabbitMQ Connection ---
	//-------------------------------------------------------
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ:", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Failed to open channel:", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"hr_raw_queue",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": "email_dlq",
		},
	)
	if err != nil {
		log.Fatal("Failed to declare queue:", err)
	}

	hr := models.HRMessage{
		HRName:      "John",
		HREmail:     "john@company.com",
		CompanyName: "Acme Corp",
		Website:     "https://acme.com",
	}

	data, err := json.Marshal(hr)
	if err != nil {
		log.Fatal("JSON marshal failed:", err)
	}

	_, err = dbConn.Exec(
		`INSERT INTO outreach_emails 
	 (company_name, hr_email, status) 
	 VALUES ($1, $2, 'pending_ai')`,
		hr.CompanyName,
		hr.HREmail,
	)

	if err != nil {
		log.Println("DB insert failed, skipping message:", err)
		// continue // IMPORTANT: don't publish if DB insert fails
	} else {
		err = ch.Publish(
			"",
			q.Name,
			false,
			false,
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        data,
				Headers: amqp.Table{
					"retry_count": int32(0),
				},
			},
		)

		if err != nil {
			log.Fatal("Failed to publish message:", err)
		}

		log.Println("Message sent:", data)
	}
}
