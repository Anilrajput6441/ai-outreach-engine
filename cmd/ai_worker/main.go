package main

import (
	"ai-outreach-engine/internal/models"
	"encoding/json"
	"errors"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

const maxRetries = 3

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Failed to open channel:", err)
	}
	defer ch.Close()

	//-------------------------------------------------------
	// --- main queue ---
	//------------------------------------------------------

	hrRawQueue, err := ch.QueueDeclare(
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
		log.Fatal("Queue declare failed:", err)
	}

	//-------------------------------------------------------
	// --- retry queue ---
	//------------------------------------------------------

	retryQueue, err := ch.QueueDeclare(
		"email_retry_queue",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": "hr_raw_queue",
			"x-message-ttl":             int32(5000), // 5 seconds
		},
	)
	if err != nil {
		log.Fatal("Retry Queue declare failed:", err)
	}

	//-------------------------------------------------------
	// --- email_send_queue ---
	//------------------------------------------------------
	_, err = ch.QueueDeclare(
		"email_send_queue",
		true,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		log.Fatal("email send queue failed to declare:", err)
	}
	//-------------------------------------------------------
	// --- dead letter queue (broker-managed) ---
	// Messages with max retries auto-route here via
	// the x-dead-letter-routing-key from main queue
	//------------------------------------------------------

	_, err = ch.QueueDeclare(
		"email_dlq",
		true,  // durable
		false, // exclusive
		false, // auto-delete
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		log.Fatal("DLQ declare failed:", err)
	}

	err = ch.Qos( //Fair dispatch---->>>	//With QoS: RabbitMQ sends only 1 at a time.
		1, //Without QoS:RabbitMQ may dump 10 messages to Worker A
		0,
		false,
	)
	if err != nil {
		log.Fatal("Failed to set QoS:", err)
	}

	//-------------------------------------------------------
	// --- Consuming ---
	//------------------------------------------------------
	msgs, err := ch.Consume(
		hrRawQueue.Name,
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

	forever := make(chan bool) // just to make it run forever ----->>>>>>>>>>>>>>>>>

	go func() {
		for d := range msgs {
			log.Println("Received message:", string(d.Body))

			// Get retry count from headers
			retryCount := int32(0)
			if val, ok := d.Headers["retry_count"]; ok {
				retryCount = val.(int32)
			}

			// Process message once
			log.Println("Processing message... (Attempt:", retryCount+1, ")")
			err := processMessage(ch, d)

			if err != nil {
				// Processing failed
				log.Println("Error processing message:", err)
				handleRetry(ch, d, retryQueue.Name)
			} else {
				// Success
				log.Println("Message processed successfully.")
				d.Ack(false)
			}
		}
	}()

	log.Println("Waiting for messages...")

	<-forever
}

func processMessage(ch *amqp.Channel, d amqp.Delivery) error {
	// Simulate processing logic
	var hr models.HRMessage
	err := json.Unmarshal(d.Body, &hr)
	if err != nil {
		log.Println("Failed to unmarshal message:", err)
		return errors.New("invalid message format")
	}

	log.Println("Generating AI email for:", hr.CompanyName)

	// 1️⃣ Call dummy AI generator
	email := generateDummyEmail(hr)

	// 2️⃣ Marshal EmailMessage
	emailData, err := json.Marshal(email)
	if err != nil {
		log.Println("Failed to marshal email:", err)
		return errors.New("failed to marshal email message")
	}

	// 3️⃣ Push to email_send_queue
	err = ch.Publish(
		"",
		"email_send_queue",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        emailData,
			Headers: amqp.Table{
				"retry_count": int32(0),
			},
		},
	)

	if err != nil {
		log.Println("Failed to publish to email queue:", err)
		return errors.New("failed to publish email message")
	}

	// Here you would:
	// - Call external API
	// - Send email
	// - Save to DB
	// - Run AI logic

	return nil
}

func generateDummyEmail(hr models.HRMessage) models.EmailMessage {
	subject := "Excited to contribute to " + hr.CompanyName

	body := "Hi " + hr.HRName + ",\n\n" +
		"I came across " + hr.CompanyName + " and was impressed by your work.\n" +
		"I believe my backend experience in Go and distributed systems would be valuable.\n\n" +
		"Looking forward to connecting.\n\n" +
		"Best,\nAnil"

	return models.EmailMessage{
		HREmail:     hr.HREmail,
		CompanyName: hr.CompanyName,
		Subject:     subject,
		Body:        body,
	}
}

func handleRetry(ch *amqp.Channel, d amqp.Delivery, retryQueueName string) {
	retryCount := int32(0)
	if val, ok := d.Headers["retry_count"]; ok {
		retryCount = val.(int32)
	}

	if retryCount < maxRetries {
		log.Println("retrying message. Attempt:", retryCount+1)
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
			log.Println("Retry publish failed. Requeueing...")
			d.Nack(false, true) // requeue to main queue
			return
		}
		d.Ack(false)
	} else {

		log.Println("Max retries reached. Sending to DLQ.")
		d.Nack(false, false) // broker sends to DLQ
	}
}
