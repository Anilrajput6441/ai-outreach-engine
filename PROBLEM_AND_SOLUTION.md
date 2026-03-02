# DB vs Queue Atomicity Problem

## 🔥 Core Problem
Your producer logic currently:

1. Insert row into database (`pending_ai` table)
2. If the insert succeeds, publish a message to RabbitMQ
3. If the insert fails, skip publishing

This seems reasonable, but if the insert fails (e.g. due to a transient DB issue or conflict), the corresponding CSV row is effectively *lost*. In a real CSV import scenario this means data disappears without trace.

The underlying issue is that PostgreSQL and RabbitMQ are independent systems. There is **no way to atomically commit a database transaction and a message broker publish** in one step.

## ❌ Naive approaches (and why they fail)

| Approach | What happens | Why it's bad |
|----------|--------------|--------------|
| Publish first, DB insert after | Message sent, DB insert may fail | Email is processed with no record in your DB. You can't track or reconcile it. Orphaned messages are hard to debug. |
| DB insert first, publish later *(your current code)* | DB insert fails → no message | No message means the CSV row is lost forever unless you manually retry. Dangerous for production. |

Both patterns expose you to data loss or inconsistent state.

## ✅ Correct industry solution: Transactional Outbox Pattern
This is the standard pattern used by large systems (Stripe, Uber, Shopify, etc.) to achieve reliable communication between a transactional database and an asynchronous message bus.

### How it works
1. Perform **all database work** in one transaction:
   - `INSERT INTO outreach_emails ... status=pending`
   - `INSERT INTO outbox_events (payload, channel, status) VALUES (..., 'rabbitmq', 'pending')`

2. Commit the transaction. Either everything succeeds or it all rolls back.

3. A separate **outbox worker** (could be another goroutine/process) repeatedly:
   - SELECT pending rows from `outbox_events`
   - Publish each payload to RabbitMQ
   - Mark the outbox row as `sent` (or delete it) in the same database transaction as marking it sent.

### Benefits
* **No data loss** – if the DB transaction fails, no message is ever created.
* **No orphan messages** – the message is only published after the DB commit.
* **Crash safe and retryable** – if publishing fails, the outbox row remains for retry.
* **At-least-once delivery** with the ability to deduplicate downstream if necessary.

### Why you couldn’t just publish directly
* You can't execute `BEGIN; INSERT ...; PUBLISH; COMMIT;` because RabbitMQ has no visibility into your DB transaction.
* There is no distributed transaction (XA) available or practical for a simple Go service talking to RabbitMQ.

### Summary
This is a classic distributed-systems challenge, not a “bug” in your code. Use a transactional outbox to tie database state changes and queue publishes into a reliable workflow.

---

