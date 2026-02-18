package main

import (
	"errors"
	"log"
	"time"

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

	mainqueue, err := ch.QueueDeclare(
		"email_queue",
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
			"x-dead-letter-routing-key": "email_queue",
			"x-message-ttl":             int32(5000), // 5 seconds
		},
	)
	if err != nil {
		log.Fatal("Retry Queue declare failed:", err)
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
		mainqueue.Name,
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

	forever := make(chan bool)

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
			err := processMessage(d.Body)

			if err != nil {
				// Processing failed
				log.Println("Error processing message:", err)

				if retryCount < maxRetries {
					// Retry: publish to retry queue with TTL
					log.Println("Retrying... Attempt:", retryCount+1)
					err := ch.Publish(
						"",
						retryQueue.Name,
						false,
						false,
						amqp.Publishing{
							ContentType: "text/plain",
							Body:        d.Body,
							Headers: amqp.Table{
								"retry_count": retryCount + 1,
							},
						},
					)
					if err != nil {
						log.Println("Failed to republish for retry:", err)
						d.Nack(false, true) // ← REQUEUE in main queue, not DLQ
					} else {
						d.Ack(false) // remove from main queue and its already moved to retry queue
					}
				} else {
					// Max retries exceeded: broker auto-sends to DLQ
					log.Println("Max retries reached. Sending to DLQ.")
					d.Nack(false, false) // Broker routes to DLQ automatically
				}
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

func processMessage(body []byte) error {
	// Simulate processing logic
	time.Sleep(2 * time.Second)

	// Example: messages with "error" text fail
	if string(body) == "error" {
		return errors.New("processing failed")
	}

	// Here you would:
	// - Call external API
	// - Send email
	// - Save to DB
	// - Run AI logic

	return nil
}
