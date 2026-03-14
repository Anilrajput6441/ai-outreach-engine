package main

import (
	"ai-outreach-engine/internal/ai"
	"ai-outreach-engine/internal/db"
	"ai-outreach-engine/internal/models"
	"database/sql"
	"encoding/json"
	"errors"
	"html"
	"log"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

const maxRetries = 3

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
		"hr_retry_queue",
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
			err := processMessage(ch, d, dbConn)

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

func processMessage(ch *amqp.Channel, d amqp.Delivery, dbConn *sql.DB) error {
	// Simulate processing logic
	var hr models.HRMessage
	err := json.Unmarshal(d.Body, &hr)
	if err != nil {
		log.Println("Failed to unmarshal message:", err)
		return errors.New("invalid message format")
	}

	log.Println("Generating AI email for:", hr.CompanyName)

	prompt := "Write a short cold email to HR named " + hr.HRName + " from company " + hr.CompanyName

	emailText, err := ai.GenerateEmail(prompt)
	if err != nil {
		log.Println("AI error:", err)
		return err
	}

	log.Printf("RAW AI RESPONSE:\n%s\n", emailText)

	email, err := parseEmailFromAI(emailText, hr)
	if err != nil {
		log.Println("Failed to parse AI email:", err)
		return err
	}
	log.Println("final email", email)

	//update db status to 'ai_generated'
	result, err := dbConn.Exec(
		`UPDATE outreach_emails 
	 SET status = 'ai_generated', ai_content = $2, updated_at = NOW() 
	 WHERE hr_email = $1`,
		hr.HREmail,
		email.Subject+"\n\n"+email.Body,
	)
	if err != nil {
		log.Println("DB update failed:", err)
	} else {
		rows, _ := result.RowsAffected()
		log.Println("DB rows affected:", rows)
	}

	//  Marshal EmailMessage
	emailData, err := json.Marshal(email)
	if err != nil {
		log.Println("Failed to marshal email:", err)
		return errors.New("failed to marshal email message")
	}

	// Push to email_send_queue
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

	return nil
}

func parseEmailFromAI(emailText string, hr models.HRMessage) (models.EmailMessage, error) {
	sections := map[string]string{}
	currentKey := ""
	var currentVal strings.Builder

	for _, line := range strings.Split(emailText, "\n") {
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "subject:"):
			currentKey = "subject"
			currentVal.Reset()
			currentVal.WriteString(strings.TrimSpace(line[len("subject:"):]))
		case strings.HasPrefix(lower, "greeting:"):
			sections[currentKey] = currentVal.String()
			currentKey = "greeting"
			currentVal.Reset()
			currentVal.WriteString(strings.TrimSpace(line[len("greeting:"):]))
		case strings.HasPrefix(lower, "body:"):
			sections[currentKey] = currentVal.String()
			currentKey = "body"
			currentVal.Reset()
			currentVal.WriteString(strings.TrimSpace(line[len("body:"):]))
		case strings.HasPrefix(lower, "closing:"):
			sections[currentKey] = currentVal.String()
			currentKey = "closing"
			currentVal.Reset()
			currentVal.WriteString(strings.TrimSpace(line[len("closing:"):]))
		default:
			if currentKey != "" && strings.TrimSpace(line) != "" {
				currentVal.WriteString("\n" + strings.TrimSpace(line))
			}
		}
	}
	if currentKey != "" {
		sections[currentKey] = currentVal.String()
	}

	subject := html.UnescapeString(sections["subject"])
	greeting := html.UnescapeString(sections["greeting"])
	body := html.UnescapeString(sections["body"])
	closing := html.UnescapeString(sections["closing"])

	log.Printf("Parsed sections -> subject:[%s] greeting:[%s] body:[%s] closing:[%s]", subject, greeting, body, closing)

	if subject == "" || body == "" {
		return models.EmailMessage{}, errors.New("failed to parse subject or body from AI response")
	}

	fullBody := greeting + "\n\n" + body + "\n\n" + closing

	return models.EmailMessage{
		HREmail:     hr.HREmail,
		CompanyName: hr.CompanyName,
		Subject:     subject,
		Body:        strings.TrimSpace(fullBody),
	}, nil
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
