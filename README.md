# Fleetwatch

A scaled-down device fleet platform: simulated cameras detect motion, stream events through Kafka into DynamoDB, and accept remote commands over SQS. Built in Go, deployed on Kubernetes, and provisioned with Terraform.

> **Status:** Early development. See [docs/DESIGN.md](docs/DESIGN.md) for the full design and milestone plan.

## Architecture

```
Device agent (Go + OpenCV) → ingest API → Kafka → consumer → DynamoDB
                                  ↓
                                Redis (rate limit, device status)

Operator → commander API → Postgres (registry/config) → SQS → device agent
```

## Tech stack

Go · Kafka (Redpanda) · Redis · PostgreSQL · DynamoDB · SQS · Docker · Kubernetes · Terraform · LocalStack · OpenCV (GoCV) · k6

## Quick start

```bash
make up        # start Postgres, Redis, Redpanda, LocalStack
make build     # build all services into ./bin
./bin/ingest   # starts on :8080

curl -X POST localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"cam-001","event_id":"e1","type":"motion","timestamp":"2026-10-14T12:00:00Z"}'
```

Full environment setup: [docs/DAY1_SETUP.md](docs/DAY1_SETUP.md).

## Repository layout

```
cmd/         service entry points (ingest, consumer, commander, agent)
internal/    shared packages (store, queue, cache, ratelimit, motion)
deploy/      Dockerfiles and Kubernetes manifests
infra/       Terraform
loadtest/    k6 scripts
docs/        design doc, setup guide, load test results
```

## Results

Load test results will be published in `docs/RESULTS.md` once measured.

## License

MIT
