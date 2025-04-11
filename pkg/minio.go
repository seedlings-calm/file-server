package pkg

import (
	"context"
	"fmt"
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

// ListFiles lists all files in a folder
func (m *MinIOClient) ListFiles(ctx context.Context, bucket, folder string) ([]minio.ObjectInfo, error) {
	objectPath := ApplyPathStrategy(folder, "")
	objects := make([]minio.ObjectInfo, 0)
	for object := range m.Client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: objectPath, Recursive: true}) {
		if object.Err != nil {
			return nil, object.Err
		}
		objects = append(objects, object)
	}
	return objects, nil
}

type MigrateResult struct {
	SuccessCount int
	Failed       []MigrateError
}

type MigrateError struct {
	ObjectKey string
	Error     string
}

// MigrateFiles migrates multiple objects from one bucket to another
// prefixes 参数类型：[]string{"images/", "videos/"} 目录迁移  []string{"static/logo.png"}文件迁移，  []string{""},nil 整个bucket迁移
func (m *MinIOClient) MigrateFiles(ctx context.Context, srcBucket, dstBucket string, prefixes []string, overwrite, removeSource bool, renameFunc func(string) string) (*MigrateResult, error) {
	result := &MigrateResult{}

	if len(prefixes) == 0 {
		prefixes = []string{""}
	}

	err := m.EnsureBucket(ctx, dstBucket)
	if err != nil {
		return nil, fmt.Errorf("目标 bucket 创建失败: %w", err)
	}

	keysToMigrate := make(map[string]struct{})
	for _, prefix := range prefixes {
		opts := minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		}
		for obj := range m.Client.ListObjects(ctx, srcBucket, opts) {
			if obj.Err != nil {
				result.Failed = append(result.Failed, MigrateError{ObjectKey: obj.Key, Error: obj.Err.Error()})
				continue
			}
			keysToMigrate[obj.Key] = struct{}{}
		}
	}

	for key := range keysToMigrate {
		dstKey := key
		if renameFunc != nil {
			dstKey = renameFunc(key)
		}

		if !overwrite {
			_, err := m.Client.StatObject(ctx, dstBucket, dstKey, minio.StatObjectOptions{})
			if err == nil {
				result.Failed = append(result.Failed, MigrateError{ObjectKey: dstKey, Error: "目标已存在,执行跳过"})
				continue
			}
		}

		src := minio.CopySrcOptions{Bucket: srcBucket, Object: key}
		dst := minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey}

		_, err := m.Client.CopyObject(ctx, dst, src)
		if err != nil {
			result.Failed = append(result.Failed, MigrateError{ObjectKey: key, Error: fmt.Sprintf("复制%s -> %s 失败: %s", key, dstKey, err.Error())})
			continue
		}
		if removeSource {
			err = m.Client.RemoveObject(ctx, srcBucket, key, minio.RemoveObjectOptions{})
			if err != nil {
				result.Failed = append(result.Failed, MigrateError{ObjectKey: key, Error: fmt.Sprintf("移除源目标的文件失败: %s", err.Error())})
				continue
			}
		}
		result.SuccessCount++
	}

	return result, nil
}
