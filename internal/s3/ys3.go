package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dnonakolesax/cccad-locks/internal/configs"
)

type Worker struct {
	S3Client *s3.Client
	bucket   string
}

// NewWorker creates an S3 worker.
func NewWorker(s3config *configs.S3Config) (*Worker, error) {
	cfg, err := config.LoadDefaultConfig(ctxWithDefault())
	if err != nil {
		return nil, err
	}

	cfg.BaseEndpoint = &s3config.Addr

	s3client := s3.NewFromConfig(cfg)
	return &Worker{S3Client: s3client, bucket: s3config.Bucket}, nil
}

func (sw *Worker) DownloadFile(ctx context.Context, objectKey string) ([]byte, error) {
	result, err := sw.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(sw.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return []byte{}, err
	}
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		return []byte{}, err
	}
	return body, nil
}

func (sw *Worker) UploadFile(ctx context.Context, objectKey string, body []byte) error {
	objectKey = strings.ReplaceAll(objectKey, "-", "")
	_, err := sw.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(sw.bucket),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(body),
	})
	if err != nil {
		return err
	}
	return nil
}

func (sw *Worker) DeleteFile(ctx context.Context, objectKey string) error {
	_, err := sw.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(sw.bucket),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		return err
	}
	return nil
}

func (sw *Worker) MoveS3Object(
	ctx context.Context,
	sourceBucket, sourceKey, destinationBucket, destinationKey string,
) error {
	copySource := url.QueryEscape(fmt.Sprintf("/%s/%s", sourceBucket, sourceKey))
	_, err := sw.S3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     &destinationBucket,
		CopySource: &copySource,
		Key:        &destinationKey,
	})
	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	_, err = sw.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &sourceBucket,
		Key:    &sourceKey,
	})
	if err != nil {
		return fmt.Errorf("failed to delete source object after copy: %w", err)
	}

	return nil
}

func ctxWithDefault() context.Context {
	return context.Background()
}
