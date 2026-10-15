# Day 1 Setup Checklist

Commands assume macOS (Homebrew) or Ubuntu/WSL2. Pick the block for your OS.

## 1. Install tooling

**macOS**
```bash
brew install go kubectl kind terraform awscli k6
brew install --cask docker
pip install awscli-local        # provides `awslocal` for LocalStack
```

**Ubuntu / WSL2**
```bash
sudo apt update && sudo apt install -y golang-go git make curl unzip
# Docker: https://docs.docker.com/engine/install/ubuntu/  (on WSL2, use Docker Desktop with WSL integration)
# kind
curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64 && chmod +x kind && sudo mv kind /usr/local/bin/
# kubectl: https://kubernetes.io/docs/tasks/tools/
# Terraform: https://developer.hashicorp.com/terraform/install
# k6: https://grafana.com/docs/k6/latest/set-up/install-k6/
pip install awscli awscli-local
```

Check everything:
```bash
go version && docker --version && kind version && kubectl version --client && terraform -version && k6 version
```
Go should be 1.22 or newer.

## 2. Create the GitHub repo

1. Create an empty repo named `fleetwatch` on GitHub (no README, no .gitignore, since the project already has them).
2. Unzip the project and push it:

```bash
cd fleetwatch
# edit go.mod: replace YOUR_GITHUB_USERNAME with your GitHub username
git init -b main
git add .
git commit -m "Initial scaffold: design doc, compose stack, ingest stub"
git remote add origin https://github.com/YOUR_GITHUB_USERNAME/fleetwatch.git
git push -u origin main
```

Tips: commit small and often. A history of many real commits over several weeks looks far more credible than one giant dump.

## 3. Start local dependencies

```bash
make up
docker compose ps        # all four containers should be running
```

## 4. Run the ingest stub

```bash
make build
./bin/ingest &
curl -i localhost:8080/healthz
curl -i -X POST localhost:8080/v1/events \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"cam-001","event_id":"e1","type":"motion","timestamp":"2026-10-14T12:00:00Z"}'
```
Expect `200` and `202`.

## 5. Verify LocalStack

```bash
awslocal sqs create-queue --queue-name smoke-test
awslocal sqs list-queues
awslocal sqs delete-queue --queue-url http://sqs.us-east-1.localhost.localstack.cloud:4566/000000000000/smoke-test
```

## 6. Learn-as-you-go order

1. Finish [A Tour of Go](https://go.dev/tour) (goroutines, channels, interfaces).
2. Phase 1: add Postgres to the ingest/commander (`pgx` library) and a migration for the `devices` table.
3. Phase 2: Kafka producer/consumer (`segmentio/kafka-go`) and DynamoDB writes (AWS SDK for Go v2, pointed at LocalStack).
4. Continue through the phases in [DESIGN.md](DESIGN.md) section 12.

## 7. Cost safety (for the final real-AWS deploy)

- Create an AWS budget and billing alarm **before** running anything real.
- Deploy once, capture screenshots and metrics, then run `terraform destroy`.

## Optional: GitHub Actions

Add `.github/workflows/ci.yml` running `go vet ./...` and `go test ./...` on every push. A green CI badge in the README is a cheap credibility signal.
