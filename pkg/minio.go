package pkg

import (
	"context"
	"io"
	"mime/multipart"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOClient struct {
	Client       *minio.Client
	Endpoint     string
	UseSSL       bool
	bucketCached sync.Map //string->bool
}

// NewMinIOClient initializes a new MinIO client
func NewMinIOClient(endpoint, accessKey, secretKey string, useSSL bool) (*MinIOClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	return &MinIOClient{
		Client:       client,
		Endpoint:     endpoint,
		UseSSL:       useSSL,
		bucketCached: sync.Map{},
	}, nil
}

// EnsureBucket makes sure the bucket exists
func (m *MinIOClient) EnsureBucket(ctx context.Context, bucket string) error {
	if _, ok := m.bucketCached.Load(bucket); ok {
		return nil
	}
	exists, err := m.Client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		err = m.Client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
	}
	m.bucketCached.Store(bucket, true)
	return nil
}

// applyPathStrategy applies a strategy to object key paths (e.g., prefixing with folder/date)
func ApplyPathStrategy(folder, objectName string) string {
	cleanFolder := strings.Trim(folder, "/")
	if cleanFolder == "" {
		return objectName
	}
	return path.Join(cleanFolder, objectName)
}

// UploadFile uploads a file to MinIO from an io.Reader
func (m *MinIOClient) UploadFile(ctx context.Context, bucket, folder, objectName string, reader io.Reader, objectSize int64, contentType string) (minio.UploadInfo, error) {
	objectPath := ApplyPathStrategy(folder, objectName)
	return m.Client.PutObject(ctx, bucket, objectPath, reader, objectSize, minio.PutObjectOptions{ContentType: contentType})
}

// UploadMultipartFile 上传multipart.File
func (m *MinIOClient) UploadMultipartFile(ctx context.Context, bucket, folder, objectName string, file multipart.File, size int64, contentType string) (minio.UploadInfo, error) {
	defer file.Close()
	return m.UploadFile(ctx, bucket, folder, objectName, file, size, contentType)
}

// UploadLargeFile supports resumable multipart upload
func (m *MinIOClient) UploadLargeFile(ctx context.Context, bucket, folder, objectName string, reader io.Reader, size int64, partSize uint64, contentType string) (minio.UploadInfo, error) {
	objectPath := ApplyPathStrategy(folder, objectName)
	opts := minio.PutObjectOptions{
		ContentType: contentType,
		PartSize:    partSize,
	}
	return m.Client.PutObject(ctx, bucket, objectPath, reader, size, opts)
}

// DownloadFile downloads an object as a stream
func (m *MinIOClient) DownloadFile(ctx context.Context, bucket, folder, objectName string) (io.ReadCloser, error) {
	objectPath := ApplyPathStrategy(folder, objectName)
	return m.Client.GetObject(ctx, bucket, objectPath, minio.GetObjectOptions{})
}

// GeneratePresignedURL creates a presigned URL for accessing a file
func (m *MinIOClient) GeneratePresignedURL(ctx context.Context, bucket, folder, objectName string, expiry time.Duration) (string, error) {
	objectPath := ApplyPathStrategy(folder, objectName)
	u, err := m.Client.PresignedGetObject(ctx, bucket, objectPath, expiry, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// DeleteFile deletes a file from MinIO
func (m *MinIOClient) DeleteFile(ctx context.Context, bucket, folder, objectName string) error {
	objectPath := ApplyPathStrategy(folder, objectName)
	return m.Client.RemoveObject(ctx, bucket, objectPath, minio.RemoveObjectOptions{})
}
