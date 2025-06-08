# Todo Service

A Go-based todo service with MySQL, Redis Streams, and S3 file storage.

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.21+

### Running the Project

```bash
# Start all services (MySQL, Redis, LocalStack S3, Todo Service)
make run

# Check service health
curl http://localhost:8083/health
```

The service will be available at `http://localhost:8083`

## Database Migrations

Migrations run automatically when MySQL container starts. SQL files are located in `mysql-init/`:

- `001_create_todos_table.sql` - Creates todos table with indexes

No manual migration steps required.

## API Endpoints

- `GET /health` - Health check
- `POST /api/v1/todo` - Create todo
- `POST /api/v1/upload` - Upload file

## Testing & Benchmarks

### Run Tests
```bash
# All tests
make test

# Generate mocks (if needed)
make generate-mocks
```

### Run Benchmarks
```bash
# All benchmarks
make benchmark

# Individual benchmarks
go test ./benchmarks -bench=BenchmarkMySQLTodoInsert -v
go test ./benchmarks -bench=BenchmarkS3FileUpload -v
go test ./benchmarks -bench=BenchmarkRedisStreamPublish -v
```


## Development Tools

```bash
# Check Redis streams
make check-redis

# Check S3 bucket contents  
make check-s3

# Stop all services
docker-compose down
```

