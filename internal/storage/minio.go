package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/intraceai/capture-node/pkg/shared"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOStorage struct {
	client     *minio.Client
	bucket     string
	publicURL  string
}

func NewMinIOStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool, publicURL string) (*MinIOStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &MinIOStorage{
		client:    client,
		bucket:    bucket,
		publicURL: publicURL,
	}, nil
}

func (s *MinIOStorage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket: %w", err)
	}

	if !exists {
		err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}

		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}]
		}`, s.bucket)

		err = s.client.SetBucketPolicy(ctx, s.bucket, policy)
		if err != nil {
			return fmt.Errorf("failed to set bucket policy: %w", err)
		}
	}

	return nil
}

func (s *MinIOStorage) StoreScreenshot(ctx context.Context, captureID string, data []byte) error {
	path := fmt.Sprintf("captures/%s/screenshot.png", captureID)
	reader := bytes.NewReader(data)

	_, err := s.client.PutObject(ctx, s.bucket, path, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "image/png",
	})
	return err
}

func (s *MinIOStorage) StoreDOM(ctx context.Context, captureID string, data []byte) error {
	path := fmt.Sprintf("captures/%s/dom.html", captureID)
	reader := bytes.NewReader(data)

	_, err := s.client.PutObject(ctx, s.bucket, path, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "text/html; charset=utf-8",
	})
	return err
}

func (s *MinIOStorage) StoreManifest(ctx context.Context, captureID string, manifest *shared.Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	path := fmt.Sprintf("captures/%s/manifest.json", captureID)
	reader := bytes.NewReader(data)

	_, err = s.client.PutObject(ctx, s.bucket, path, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	return err
}

func (s *MinIOStorage) StoreEvent(ctx context.Context, captureID string, event *shared.CaptureEvent) error {
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return err
	}

	path := fmt.Sprintf("captures/%s/event.json", captureID)
	reader := bytes.NewReader(data)

	_, err = s.client.PutObject(ctx, s.bucket, path, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	return err
}

func (s *MinIOStorage) GetScreenshot(ctx context.Context, captureID string) ([]byte, error) {
	path := fmt.Sprintf("captures/%s/screenshot.png", captureID)
	return s.getObject(ctx, path)
}

func (s *MinIOStorage) GetDOM(ctx context.Context, captureID string) ([]byte, error) {
	path := fmt.Sprintf("captures/%s/dom.html", captureID)
	return s.getObject(ctx, path)
}

func (s *MinIOStorage) GetManifest(ctx context.Context, captureID string) (*shared.Manifest, error) {
	path := fmt.Sprintf("captures/%s/manifest.json", captureID)
	data, err := s.getObject(ctx, path)
	if err != nil {
		return nil, err
	}

	var manifest shared.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (s *MinIOStorage) GetEvent(ctx context.Context, captureID string) (*shared.CaptureEvent, error) {
	path := fmt.Sprintf("captures/%s/event.json", captureID)
	data, err := s.getObject(ctx, path)
	if err != nil {
		return nil, err
	}

	var event shared.CaptureEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

func (s *MinIOStorage) getObject(ctx context.Context, path string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	return io.ReadAll(obj)
}

func (s *MinIOStorage) GetScreenshotURL(captureID string) string {
	return fmt.Sprintf("%s/%s/captures/%s/screenshot.png", s.publicURL, s.bucket, captureID)
}

func (s *MinIOStorage) GetPresignedScreenshotURL(ctx context.Context, captureID string, expiry time.Duration) (string, error) {
	path := fmt.Sprintf("captures/%s/screenshot.png", captureID)
	url, err := s.client.PresignedGetObject(ctx, s.bucket, path, expiry, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

func (s *MinIOStorage) CaptureExists(ctx context.Context, captureID string) (bool, error) {
	path := fmt.Sprintf("captures/%s/manifest.json", captureID)
	_, err := s.client.StatObject(ctx, s.bucket, path, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
