package ossclient

import (
	"context"
	"io"
	"time"
)

type OssClient interface {
	Upload(ctx context.Context, bucket, object string, data []byte, opts ...UploadOpt) error
	UploadFile(ctx context.Context, bucket, object string, filePath string, _ ...UploadOpt) error
	UploadFromReader(ctx context.Context, bucket, object string, reader io.Reader, _ ...UploadOpt) error
	Download(ctx context.Context, bucket, object string) ([]byte, error)
	DownloadFile(ctx context.Context, bucket, object string, filePath string) error
	DownloadTo(ctx context.Context, bucket, object string, w io.Writer) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
	EnsureBucketExists(ctx context.Context, bucket string) error
	GetObjectInfo(ctx context.Context, bucket, object string) (*ObjectInfo, error)
	DeleteObject(ctx context.Context, bucket, object string) error
}

/*---------- 空选项占位 ----------*/
type UploadOpt struct{}

// ObjectInfo 通用元信息结构
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
}
