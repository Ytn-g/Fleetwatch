# Fleetwatch — Design Document

**Status:** Draft v1 · **Author:** Gabriel Ajibola · **Last updated:** 2026-10-14

## 1. Summary

Fleetwatch is a scaled-down device fleet platform. Simulated camera devices run motion detection and stream events to a cloud backend. Operators can remotely reconfigure devices (for example, change motion sensitivity or reboot). The backend is built to stay correct under retries, duplicates, and bursts of traffic from thousands of devices.

The project exists to practice, end to end, the patterns used in large IoT and security-camera backends: high-volume ingestion, event streaming, low-latency state caches, durable event storage, remote command delivery, container orchestration, and infrastructure as code.

## 2. Goals and non-goals

**Goals**
- Ingest events from 1,000+ simulated devices with measured throughput and p99 latency.
- Deliver at-least-once with idempotent processing, so duplicates never create duplicate records.
- Deliver remote commands to individual devices reliably, with retries and a dead-letter queue.
- Run every service in containers on Kubernetes with autoscaling.
- Define all cloud resources with Terraform; develop locally against LocalStack.
- Publish measured results (load test numbers), not estimates.

**Non-goals**
- A production-grade UI, real video streaming or storage, multi-region deployment, or real device hardware.
- Authentication beyond a simple per-device API key (documented as a known gap in section 11).

## 3. Architecture

```
                    ┌──────────────────────────┐
  Go device agent → │  ingest (HTTP, Go)       │ → Redis  (rate limit, last-seen/status)
  (OpenCV motion)   │  validate → rate limit   │
                    └────────────┬─────────────┘
                                 │ publish (key = device_id)
                                 ▼
                    Kafka topic: device.events
                                 │
                                 ▼
                    ┌──────────────────────────┐
                    │ consumer (Go)            │ → DynamoDB: events (idempotent write)
                    │ consumer group, retries  │ → DLQ topic: device.events.dlq
                    └──────────────────────────┘

  Operator → commander (HTTP, Go) → Postgres (device registry, desired config)
                    │
                    └→ SQS queue per device (commands) ← polled by device agent
                                   └→ SQS dead-letter queue
```

## 4. Services

| Service | Responsibility |
|---|---|
| `cmd/ingest` | Accepts `POST /v1/events`. Validates, applies per-device rate limiting, updates device status in Redis, publishes to Kafka, returns 202. |
| `cmd/consumer` | Consumes `device.events` in a consumer group. Writes to DynamoDB with a conditional write on `event_id`. Failed messages go to a DLQ topic after N retries. |
| `cmd/commander` | Operator API: register devices, set desired config, send commands. Registry and config live in Postgres; commands are enqueued to SQS. |
| `cmd/agent` | Simulated device. Runs OpenCV motion detection on a sample video, emits events, sends heartbeats, polls its SQS queue, applies config, and acknowledges commands. |

## 5. Data model

**Postgres (registry and config)**
- `devices(id uuid pk, name text, api_key_hash text, created_at timestamptz)`
- `device_config(device_id fk, sensitivity int, updated_at timestamptz, version int)`
- `commands(id uuid pk, device_id fk, type text, status text, created_at, acked_at)`

**DynamoDB (event history)**
- Table `events`: partition key `device_id` (S), sort key `ts#event_id` (S).
- Sorting by timestamp gives efficient "latest N events for device X" queries.
- A conditional put (`attribute_not_exists`) on the full key makes writes idempotent.
- Known tradeoff: a single very chatty device concentrates writes on one partition. Mitigation noted in section 11.

**Redis (hot state)**
- `device:{id}:status` → last-seen timestamp and status, with a TTL used to detect offline devices.
- `ratelimit:{id}` → token bucket counters.

## 6. API

- `POST /v1/events` → 202 Accepted. Body: `{device_id, event_id, type, timestamp, payload}`. 400 on invalid input, 429 when rate limited.
- `POST /v1/devices` → register a device.
- `PUT /v1/devices/{id}/config` → set desired config (bumps `version`).
- `POST /v1/devices/{id}/commands` → enqueue a command (`reboot`, `set_sensitivity`).
- `GET /v1/devices/{id}/status` → status from Redis.
- `GET /v1/devices/{id}/events?limit=N` → recent events from DynamoDB.
- `GET /healthz` on every service.

## 7. Reliability and distributed-systems design

- **Delivery semantics:** at-least-once end to end. Producers retry; consumers may see duplicates.
- **Idempotency:** every event carries a device-generated `event_id`. The DynamoDB conditional write turns duplicates into no-ops.
- **Ordering:** Kafka messages are keyed by `device_id`, so one device's events stay ordered within a partition.
- **Backpressure:** the ingest service returns 429 (via Redis token bucket) instead of letting one device overload the system.
- **Failure handling:** bounded retries with exponential backoff and jitter; poison messages go to a DLQ; SQS commands that fail repeatedly land in an SQS dead-letter queue.
- **Graceful shutdown:** every service handles SIGTERM, stops accepting work, drains in-flight messages, and commits offsets before exiting.
- **Command acknowledgment:** a command stays invisible in SQS until the agent acks; otherwise it reappears after the visibility timeout. Commands must therefore be idempotent (for example, "set sensitivity to 7", not "increase sensitivity").

## 8. Deployment

- Each service has a multi-stage Dockerfile producing a small static binary image.
- Kubernetes (kind locally): one Deployment per service, a Service for ingest, ConfigMaps for settings, readiness and liveness probes on `/healthz`.
- A HorizontalPodAutoscaler on ingest and consumer, driven by CPU (and consumer lag as a stretch goal).
- **Terraform** defines the SQS queues and DLQ, the DynamoDB table, and IAM roles. The same code targets LocalStack for development and real AWS for one final deploy.

## 9. Observability

- Structured JSON logs (`log/slog`) with `device_id` and `event_id` fields.
- Prometheus metrics endpoint on each service: request rate, latency histogram, Kafka consumer lag, DLQ depth (stretch goal).

## 10. Testing and load testing

- Unit tests for validation, rate limiter, and idempotent write logic (`go test -race`).
- Integration tests against Docker Compose dependencies.
- **Load test (k6):** ramp from 100 to 1,000+ simulated devices. Record sustained events/sec, p50/p95/p99 latency, error rate, and consumer lag. Results are committed to `docs/RESULTS.md`. **Do not publish numbers that were not measured.**
- Failure tests: kill a consumer pod mid-stream and confirm no lost or duplicated events.

## 11. Known limitations and future work

- Per-device API keys only; no mTLS or token rotation.
- Hot-partition risk in DynamoDB for very chatty devices (could add a time-bucket suffix to the partition key).
- One SQS queue per device does not scale to millions of devices; a real system would use topic-based fan-out or a device gateway such as MQTT.
- No real video pipeline; motion detection runs on a sample file.

## 12. Milestones

| Phase | Deliverable | Target |
|---|---|---|
| 1 | Go ingest API + Postgres registry in Docker Compose | Week 1 |
| 2 | Kafka publish + consumer writing to DynamoDB (LocalStack) | Week 2 |
| 3 | Redis status cache and rate limiting | Week 2-3 |
| 4 | SQS command queue + Go device agent with OpenCV motion detection | Week 3-4 |
| 5 | Kubernetes manifests + HPA, then Terraform | Week 5 |
| 6 | Load test, failure tests, write-up in `docs/RESULTS.md` | Week 6 |

## 13. Tech stack

Go 1.22+, GoCV (OpenCV), Kafka API (Redpanda locally), Redis, PostgreSQL, DynamoDB, SQS, Docker, Kubernetes (kind), Terraform, LocalStack, k6, GitHub Actions (for `go vet` and `go test`).
