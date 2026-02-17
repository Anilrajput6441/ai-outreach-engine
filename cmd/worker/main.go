package main

import (
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

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

	q, err := ch.QueueDeclare(
		"email_queue",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Queue declare failed:", err)
	}

	err = ch.Qos( //Fair dispatch---->>>	//With QoS: RabbitMQ sends only 1 at a time.
		1, //Without QoS:RabbitMQ may dump 10 messages to Worker A
		0,
		false,
	)
	if err != nil {
		log.Fatal("Failed to set QoS:", err)
	}

	msgs, err := ch.Consume(
		q.Name,
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
			d.Ack(false)
		}
	}()

	log.Println("Waiting for messages...")

	<-forever
}
