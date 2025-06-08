package benchmarks

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"todo-service/internal/config"
	"todo-service/internal/infrastructure/storage"
)

func BenchmarkS3FileUpload(b *testing.B) {
	s3Storage := setupS3Storage(b)
	defer cleanupS3TestData(b, s3Storage)
	ctx := context.Background()

	fileSizes := []struct {
		name string
		size int64
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
	}

	for _, fileSize := range fileSizes {
		b.Run(fileSize.name, func(b *testing.B) {
			testData := generateTestData(fileSize.size)

			b.ResetTimer()
			b.SetBytes(fileSize.size)

			for i := 0; i < b.N; i++ {
				storagePath := fmt.Sprintf("benchmark/test-file-%d-%d.txt", time.Now().UnixNano(), i)
				reader := bytes.NewReader(testData)

				err := s3Storage.UploadFile(ctx, storagePath, "text/plain", reader, fileSize.size)
				if err != nil {
					b.Fatalf("Failed to upload file: %v", err)
				}
			}
		})
	}
}

func BenchmarkS3FileUploadConcurrent(b *testing.B) {
	s3Storage := setupS3Storage(b)
	defer cleanupS3TestData(b, s3Storage)
	ctx := context.Background()

	concurrencyLevels := []int{1, 5, 10, 20, 50}
	fileSize := int64(100 * 1024)
	testData := generateTestData(fileSize)

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			b.SetBytes(fileSize)

			var wg sync.WaitGroup
			semaphore := make(chan struct{}, concurrency)

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()
					semaphore <- struct{}{}
					defer func() { <-semaphore }()

					storagePath := fmt.Sprintf("benchmark/concurrent-test-file-%d-%d.txt", time.Now().UnixNano(), index)
					reader := bytes.NewReader(testData)

					err := s3Storage.UploadFile(ctx, storagePath, "text/plain", reader, fileSize)
					if err != nil {
						b.Errorf("Failed to upload file: %v", err)
					}
				}(i)
			}

			wg.Wait()
		})
	}
}

func BenchmarkS3FileUploadDifferentContentTypes(b *testing.B) {
	s3Storage := setupS3Storage(b)
	defer cleanupS3TestData(b, s3Storage)
	ctx := context.Background()

	contentTypes := []struct {
		name        string
		contentType string
		extension   string
	}{
		{"Text", "text/plain", ".txt"},
		{"JSON", "application/json", ".json"},
		{"Image", "image/jpeg", ".jpg"},
		{"PDF", "application/pdf", ".pdf"},
		{"Binary", "application/octet-stream", ".bin"},
	}

	fileSize := int64(50 * 1024)

	for _, ct := range contentTypes {
		b.Run(ct.name, func(b *testing.B) {
			testData := generateTestData(fileSize)

			b.ResetTimer()
			b.SetBytes(fileSize)

			for i := 0; i < b.N; i++ {
				storagePath := fmt.Sprintf("benchmark/content-type-test-%d-%d%s", time.Now().UnixNano(), i, ct.extension)
				reader := bytes.NewReader(testData)

				err := s3Storage.UploadFile(ctx, storagePath, ct.contentType, reader, fileSize)
				if err != nil {
					b.Fatalf("Failed to upload file with content type %s: %v", ct.contentType, err)
				}
			}
		})
	}
}

func BenchmarkS3FileUploadWithTimeout(b *testing.B) {
	s3Storage := setupS3Storage(b)
	defer cleanupS3TestData(b, s3Storage)

	timeouts := []struct {
		name    string
		timeout time.Duration
	}{
		{"1s", 1 * time.Second},
		{"5s", 5 * time.Second},
		{"10s", 10 * time.Second},
		{"30s", 30 * time.Second},
	}

	fileSize := int64(1024 * 1024)
	testData := generateTestData(fileSize)

	for _, timeout := range timeouts {
		b.Run(timeout.name, func(b *testing.B) {
			b.ResetTimer()
			b.SetBytes(fileSize)

			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), timeout.timeout)

				storagePath := fmt.Sprintf("benchmark/timeout-test-%d-%d.txt", time.Now().UnixNano(), i)
				reader := bytes.NewReader(testData)

				err := s3Storage.UploadFile(ctx, storagePath, "text/plain", reader, fileSize)
				cancel()

				if err != nil {
					b.Fatalf("Failed to upload file with timeout %v: %v", timeout.timeout, err)
				}
			}
		})
	}
}

func setupS3Storage(b *testing.B) *storage.S3FileStorage {
	cfg := config.Load()

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(cfg.AWS.Region),
		Endpoint:         aws.String(cfg.AWS.Endpoint),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
	})
	if err != nil {
		b.Fatalf("Failed to create AWS session: %v", err)
	}

	s3Storage, err := storage.NewS3FileStorage(sess, cfg.AWS.S3Bucket)
	if err != nil {
		b.Fatalf("Failed to create S3 storage: %v", err)
	}

	cleanupS3TestData(b, s3Storage)

	return s3Storage
}

func cleanupS3TestData(b *testing.B, s3Storage *storage.S3FileStorage) {
	cfg := config.Load()

	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(cfg.AWS.Region),
		Endpoint:         aws.String(cfg.AWS.Endpoint),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials("test", "test", ""),
	})
	if err != nil {
		b.Logf("Warning: Failed to create AWS session for cleanup: %v", err)
		return
	}

	s3Client := s3.New(sess)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefixes := []string{
		"benchmark/",
		"test/",
		"Test/",
	}

	for _, prefix := range prefixes {
		err := cleanupS3ObjectsWithPrefix(ctx, b, s3Client, cfg.AWS.S3Bucket, prefix)
		if err != nil {
			b.Logf("Warning: Failed to cleanup S3 objects with prefix '%s': %v", prefix, err)
		}
	}
}

func cleanupS3ObjectsWithPrefix(ctx context.Context, b *testing.B, s3Client *s3.S3, bucket, prefix string) error {
	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}

	var objectsToDelete []*s3.ObjectIdentifier

	err := s3Client.ListObjectsV2PagesWithContext(ctx, listInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			objectsToDelete = append(objectsToDelete, &s3.ObjectIdentifier{
				Key: obj.Key,
			})
		}
		return !lastPage
	})

	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objectsToDelete) == 0 {
		return nil
	}

	batchSize := 1000
	for i := 0; i < len(objectsToDelete); i += batchSize {
		end := i + batchSize
		if end > len(objectsToDelete) {
			end = len(objectsToDelete)
		}

		batch := objectsToDelete[i:end]
		deleteInput := &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &s3.Delete{
				Objects: batch,
				Quiet:   aws.Bool(true),
			},
		}

		_, err := s3Client.DeleteObjectsWithContext(ctx, deleteInput)
		if err != nil {
			b.Logf("Warning: Failed to delete S3 objects batch: %v", err)
			continue
		}

		b.Logf("Cleaned up %d S3 objects with prefix '%s'", len(batch), prefix)
	}

	return nil
}

func generateTestData(size int64) []byte {
	data := make([]byte, size)

	rand.Seed(time.Now().UnixNano())
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}

	return data
}
