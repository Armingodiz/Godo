package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

type S3FileStorage struct {
	client *s3.S3
	bucket string
}

func NewS3FileStorage(sess *session.Session, bucket string) *S3FileStorage {
	return &S3FileStorage{
		client: s3.New(sess),
		bucket: bucket,
	}
}

func (s *S3FileStorage) UploadFile(ctx context.Context, storagePath, contentType string, data io.Reader, size int64) error {
	buf := make([]byte, size)
	_, err := io.ReadFull(data, buf)
	if err != nil {
		return fmt.Errorf("failed to read file data: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(storagePath),
		Body:          bytes.NewReader(buf),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}

	_, err = s.client.PutObjectWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return nil
}

func (s *S3FileStorage) EnsureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})

	if err != nil {
		_, err = s.client.CreateBucketWithContext(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(s.bucket),
		})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return nil
}
