# 🐰 Go + RabbitMQ Worker System

Simple message queue system using Go and RabbitMQ.

---

# 📦 What This Project Does

- `producer.go` → Sends messages to RabbitMQ queue
- `worker.go` → Receives and processes messages from queue
- RabbitMQ → Stores messages until a worker processes them

---

# 🛠 Requirements

- Go installed (1.20+ recommended)
- Docker installed

---

# 🐳 Step 1: Start RabbitMQ

Run this command:

```bash
docker run -d --hostname rabbit --name rabbitmq \
-p 5672:5672 -p 15672:15672 \
rabbitmq:3-management
```

This starts RabbitMQ locally.

RabbitMQ runs on:

```
amqp://guest:guest@localhost:5672/
```

Management dashboard:

```
http://localhost:15672
```

Login:
```
guest
guest
```

---

# 📥 Step 2: Install Dependencies

Inside project folder:

```bash
go mod tidy
```

---

# ▶️ Step 3: Run Worker (Receiver)

Open terminal 1:

```bash
go run worker.go
```

Worker will start waiting for messages.

You can run multiple workers to test scaling:

```bash
go run worker.go
go run worker.go
```

---

# 📤 Step 4: Run Producer (Sender)

Open terminal 2:

```bash
go run producer.go
```

Producer sends a message to the queue.

Worker will receive and process it.

---

# 🔄 How It Works

1. Producer sends message to queue
2. RabbitMQ stores message
3. Worker consumes message
4. Worker processes it
5. Worker sends ACK
6. Message removed from queue

If worker crashes before ACK → message is requeued.

---

# 🛑 Stop RabbitMQ

```bash
docker stop rabbitmq
docker rm rabbitmq
```
