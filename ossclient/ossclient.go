package ossclient

import (
	"context"
	"io"

	"github.com/hellobchain/oss-go-sdk/common/models"
)

type OssClient interface {
	Upload(ctx context.Context, bucket, object string, data []byte, opts ...models.UploadOptions) error
	UploadFile(ctx context.Context, bucket, object string, filePath string, _ ...models.UploadOptions) error
	UploadFromReader(ctx context.Context, bucket, object string, reader io.Reader, _ ...models.UploadOptions) error
	Download(ctx context.Context, bucket, object string) ([]byte, error)
	DownloadFile(ctx context.Context, bucket, object string, filePath string) error
	DownloadTo(ctx context.Context, bucket, object string, w io.Writer) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
	EnsureBucketExists(ctx context.Context, bucket string) error
	GetObjectInfo(ctx context.Context, bucket, object string) (*models.ObjectInfo, error)
	DeleteObject(ctx context.Context, bucket, object string) error
	SetBucket(bucket string)
	ObjectExists(ctx context.Context, bucket, object string) (bool, error)
}
