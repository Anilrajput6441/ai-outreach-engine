package main

import (
	"ai-outreach-engine/internal/models"
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

// maxRetries := 3
func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Println("Failed to connect to RabbitMQ:", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Println("Failed to open a channel:", err)
		return
	}
	defer ch.Close()

	//-------------------------------------------------------
	// --- email_send_queue (main) ---
	//-------------------------------------------------------
	emailQueue, err := ch.QueueDeclare(
		"email_send_queue",
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
		log.Fatal("Failed to declare email_send_queue:", err)
	}

	//-------------------------------------------------------
	// --- email_retry_queue ---
	//-------------------------------------------------------
	retryQueue, err := ch.QueueDeclare(
		"email_retry_queue",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": "email_send_queue",
			"x-message-ttl":             int32(5000), // 5 sec retry delay
		},
	)
	if err != nil {
		log.Fatal("Failed to declare email_retry_queue:", err)
	}

	//-------------------------------------------------------
	// --- email_dlq ---
	//-------------------------------------------------------
	_, err = ch.QueueDeclare(
		"email_dlq",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Failed to declare email_dlq:", err)
	}

	//-------------------------------------------------------
	// --- QoS (fair dispatch) ---
	//-------------------------------------------------------
	err = ch.Qos(1, 0, false)
	if err != nil {
		log.Fatal("Failed to set QoS:", err)
	}

	//-------------------------------------------------------
	// --- Consume ---
	//-------------------------------------------------------
	msgs, err := ch.Consume(
		emailQueue.Name,
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Failed to register consumer:", err)
	}

	log.Println("Email worker started. Waiting for messages...")

	//-------------------------------------------------------
	// --- Rate Limiter ---
	//-------------------------------------------------------
	rateLimiter := time.NewTicker(10 * time.Second) // 1 email / 10 sec
	defer rateLimiter.Stop()

	forever := make(chan bool)

	go func() {
		for d := range msgs {

			<-rateLimiter.C // rate limit enforced here

			err := processEmail(d)

			if err != nil {
				log.Println("Email send failed:", err)
				handleRetry(ch, d, retryQueue.Name)
			} else {
				d.Ack(false)
			}
		}
	}()

	<-forever
}

// -------------------------------------------------------
// Business Logic (NO ACK/NACK here)
// -------------------------------------------------------

func processEmail(d amqp.Delivery) error {
	var email models.EmailMessage

	err := json.Unmarshal(d.Body, &email)
	if err != nil {
		log.Println("failed to unmarshall")
		return err
	}

	log.Println("Sending email to:", email.HREmail)
	log.Println("Subject:", email.Subject)

	// Simulate email sending delay
	time.Sleep(2 * time.Second)

	// Simulate failure (for testing retry)
	if email.CompanyName == "fail" {
		return errors.New("SMTP error")
	}

	log.Println("Email sent successfully to:", email.HREmail)
	return nil
}

// -------------------------------------------------------
// Retry Handler
// -------------------------------------------------------

func handleRetry(ch *amqp.Channel, d amqp.Delivery, retryQueueName string) {

	retryCount := int32(0)
	if val, ok := d.Headers["retry_count"]; ok {
		if count, ok := val.(int32); ok {
			retryCount = count
		}
	}

	if retryCount < 3 {

		log.Println("Retrying email. Attempt:", retryCount+1)

		err := ch.Publish(
			"",
			retryQueueName,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        d.Body,
				Headers: amqp.Table{
					"retry_count": retryCount + 1,
				},
			},
		)
		if err != nil {
			log.Println("Failed to publish retry message:", err)
		}
		d.Ack(false)
	} else {
		log.Println("Max retries reached. Sending to DLQ.")
		d.Nack(false, false)
	}
}

// -------------------------------------------------------
// canSendEmail
// -------------------------------------------------------

func canSendEmail(ctx context.Context, rdb *redis.Client) (bool, error) {
	today := time.Now().Format("2006-01-02")
	key := "email_sent_count:" + today

	count, err := rdb.Get(ctx, key).Int()
	if err == redis.Nil {
		return true, nil // no emails sent today yet
	}
	if err != nil {
		return false, err
	}

	if count >= 20 {
		return false, nil // max emails sent for today
	}
	return true, nil

}
