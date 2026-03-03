package producer

import (
	"database/sql"
	"encoding/json"
	"log"

	"ai-outreach-engine/internal/models"

	amqp "github.com/rabbitmq/amqp091-go"
)

func Process(rows []models.HRMessage, dbConn *sql.DB, ch *amqp.Channel) {

	for _, hr := range rows {

		data, err := json.Marshal(hr)
		if err != nil {
			log.Println("JSON marshal failed:", err)
			continue
		}

		_, err = dbConn.Exec(
			`INSERT INTO outreach_emails 
			 (company_name, hr_email, status) 
			 VALUES ($1, $2, 'pending_ai')`,
			hr.CompanyName,
			hr.HREmail,
		)

		if err != nil {
			log.Println("DB insert failed, skipping:", err)
			continue
		}

		err = ch.Publish(
			"",
			"hr_raw_queue",
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        data,
				Headers: amqp.Table{
					"retry_count": int32(0),
				},
			},
		)

		if err != nil {
			log.Println("Failed to publish:", err)
			continue
		}

		log.Println("Message sent:", hr.HREmail)
	}
}
