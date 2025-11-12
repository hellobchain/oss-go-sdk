package impl

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hellobchain/oss-go-sdk/common/models"
	"github.com/hellobchain/oss-go-sdk/ossclient"
)

type localClient struct {
	dir    string
	logger ossclient.Logger
}

// DeleteObject implements ossclient.OssClient.
func (c *localClient) DeleteObject(ctx context.Context, bucket string, object string) error {
	filePath := filepath.Join(c.dir, object)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.logger.Print("file not exists")
		return err
	}
	err := os.Remove(filePath)
	if err != nil {
		c.logger.Print("delete file failed", err)
	}
	return nil
}

// Download implements ossclient.OssClient.
func (c *localClient) Download(ctx context.Context, bucket string, object string) ([]byte, error) {
	filePath := filepath.Join(c.dir, object)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.logger.Print("file not exists")
		return nil, err
	}
	return os.ReadFile(filePath)
}

// DownloadFile implements ossclient.OssClient.
func (c *localClient) DownloadFile(ctx context.Context, bucket string, object string, filePath string) error {
	fd, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(0664))
	if err != nil {
		c.logger.Print("failed to open file", filePath)
		return err
	}
	defer fd.Close()
	data, err := c.Download(ctx, bucket, object)
	if err != nil {
		c.logger.Print("failed to download file", object)
		return err
	}
	_, err = fd.Write(data)
	if err != nil {
		c.logger.Print("failed to write file", filePath)
		return err
	}
	return nil
}

// DownloadTo implements ossclient.OssClient.
func (c *localClient) DownloadTo(ctx context.Context, bucket string, object string, w io.Writer) error {
	data, err := c.Download(ctx, bucket, object)
	if err != nil {
		c.logger.Print("failed to download file", object)
		return err
	}
	_, err = w.Write(data)
	if err != nil {
		c.logger.Print("failed to write file", object)
		return err
	}
	return nil
}

// EnsureBucketExists implements ossclient.OssClient.
func (c *localClient) EnsureBucketExists(ctx context.Context, bucket string) error {
	return nil
}

// GetObjectInfo implements ossclient.OssClient.
func (c *localClient) GetObjectInfo(ctx context.Context, bucket string, object string) (*models.ObjectInfo, error) {
	filePath := filepath.Join(c.dir, object)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.logger.Print("file not exists")
		return nil, err
	}
	return &models.ObjectInfo{
		ContentType:  "application/octet-stream",
		ETag:         "etag",
		Key:          object,
		LastModified: time.Now().Format(time.RFC3339),
		Size:         0,
		UserMetadata: map[string]string{},
	}, nil
}

// ListObjects implements ossclient.OssClient.
func (c *localClient) ListObjects(ctx context.Context, bucket string, prefix string) ([]string, error) {
	return []string{}, nil
}

// ObjectExists implements ossclient.OssClient.
func (c *localClient) ObjectExists(ctx context.Context, bucket string, object string) (bool, error) {
	filePath := filepath.Join(c.dir, object)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.logger.Print("file not exists")
		return false, err
	}
	return true, nil
}

// Upload implements ossclient.OssClient.
func (c *localClient) Upload(ctx context.Context, bucket string, object string, data []byte, opts ...models.UploadOptions) error {
	filePath := filepath.Join(c.dir, object)
	if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
		c.logger.Print("failed to create directory", err)
		return err
	}
	out, err := os.Create(filePath)
	if err != nil {
		c.logger.Print("failed to create file", err)
		return err
	}
	defer out.Close()
	_, err = out.Write(data)
	if err != nil {
		c.logger.Print("failed to write file", err)
		return err
	}
	return nil
}

// UploadFile implements ossclient.OssClient.
func (c *localClient) UploadFile(ctx context.Context, bucket string, object string, filePath string, _ ...models.UploadOptions) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.logger.Print("file not exists")
		return err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		c.logger.Print("failed to read file", err)
		return err
	}
	return c.Upload(ctx, bucket, object, data)
}

// UploadFromReader implements ossclient.OssClient.
func (c *localClient) UploadFromReader(ctx context.Context, bucket string, object string, reader io.Reader, _ ...models.UploadOptions) error {
	filePath := filepath.Join(c.dir, object)
	if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
		c.logger.Print("failed to create directory", err)
		return err
	}
	out, err := os.Create(filePath)
	if err != nil {
		c.logger.Print("failed to create file", err)
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, reader)
	if err != nil {
		c.logger.Print("failed to write file", err)
		return err
	}
	return nil
}

// SetLogger implements ossclient.OssClient.
func (c *localClient) SetLogger(logger ossclient.Logger) {
	c.logger = logger
}

// SetBucket implements ossclient.OssClient.
func (c *localClient) SetBucket(bucket string) {
}

func NewLocalClient(clientConfig *models.Config) (ossclient.OssClient, error) {
	return &localClient{
		dir:    clientConfig.Dir,
		logger: &ossclient.DefaultLogger{}}, nil
}
