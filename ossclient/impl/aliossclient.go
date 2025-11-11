package impl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/hellobchain/oss-go-sdk/common/errors"
	"github.com/hellobchain/oss-go-sdk/common/models"
	"github.com/hellobchain/oss-go-sdk/ossclient"
)

/*---------- 阿里云实现 ----------*/
type aliClient struct {
	cli    *oss.Client
	bucket string
}

// SetBucket implements ossclient.OssClient.
func (c *aliClient) SetBucket(bucket string) {
	c.bucket = bucket
}

func NewAliClient(clientConfig *models.Config) (ossclient.OssClient, error) {
	if clientConfig.Endpoint == "" || clientConfig.AccessKeyID == "" || clientConfig.SecretAccessKey == "" {
		return nil, errors.ErrInvalidConfig
	}
	_, err := url.Parse(clientConfig.Endpoint)
	if err != nil {
		return nil, err
	}
	cli, err := oss.New(clientConfig.Endpoint, clientConfig.AccessKeyID, clientConfig.SecretAccessKey, oss.InsecureSkipVerify(true))
	if err != nil {
		return nil, err
	}
	if clientConfig.Region == "" {
		cli.SetRegion(clientConfig.Region)
	}
	aliClient := &aliClient{cli: cli, bucket: clientConfig.BucketName}
	if clientConfig.BucketName != "" {
		err = aliClient.EnsureBucketExists(context.Background(), clientConfig.BucketName)
		if err != nil {
			return nil, err
		}
	}
	return aliClient, nil
}

func (c *aliClient) Upload(ctx context.Context, bucket, object string, data []byte, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return err
	}
	return bkt.PutObject(object, bytes.NewReader(data))
}

func (c *aliClient) UploadFile(ctx context.Context, bucket, object string, filePath string, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return errors.ErrClientNotInitialized
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return err
	}
	return bkt.PutObjectFromFile(object, filePath)
}
func (c *aliClient) UploadFromReader(ctx context.Context, bucket, object string, reader io.Reader, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return err
	}
	return bkt.PutObject(object, reader)
}

func (c *aliClient) Download(ctx context.Context, bucket, object string) ([]byte, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return nil, errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return nil, err
	}
	out, err := bkt.GetObject(object)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	return io.ReadAll(out)
}
func (c *aliClient) DownloadFile(ctx context.Context, bucket, object string, filePath string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return err
	}
	return bkt.GetObjectToFile(object, filePath)
}

func (c *aliClient) DownloadTo(ctx context.Context, bucket, object string, w io.Writer) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return err
	}
	rc, err := bkt.GetObject(object)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(w, rc)
	return err
}
func (c *aliClient) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return nil, errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return nil, err
	}
	marker := ""
	var keys []string
	for {
		lr, err := bkt.ListObjects(oss.Prefix(prefix), oss.Marker(marker))
		if err != nil {
			return nil, err
		}
		for _, o := range lr.Objects {
			keys = append(keys, o.Key)
		}
		if !lr.IsTruncated {
			break
		}
		marker = lr.NextMarker
	}
	return keys, nil
}

func (c *aliClient) EnsureBucketExists(ctx context.Context, bucket string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return errors.ErrClientNotInitialized
	}
	exist, err := c.cli.IsBucketExist(bucket)
	if err != nil {
		return fmt.Errorf("check bucket exist: %w", err)
	}
	if exist {
		return nil
	}
	// 创建，ACL 默认私有
	return c.cli.CreateBucket(bucket, oss.ACL(oss.ACLPrivate))
}

func (c *aliClient) GetObjectInfo(ctx context.Context, bucket, object string) (*models.ObjectInfo, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return nil, errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return nil, err
	}
	meta, err := bkt.GetObjectDetailedMeta(object)
	if err != nil {
		return nil, err
	}
	// 解析头域
	size := meta.Get("Content-Length")
	lm := meta.Get("Last-Modified")
	etag := meta.Get("ETag")
	sz, _ := strconv.ParseInt(size, 10, 64)
	return &models.ObjectInfo{
		Key:          object,
		Size:         sz,
		LastModified: lm,
		ETag:         strings.Trim(etag, `"`),
		ContentType:  meta.Get("Content-Type"),
	}, nil
}

func (c *aliClient) DeleteObject(ctx context.Context, bucket, object string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return err
	}
	return bkt.DeleteObject(object)
}

func (c *aliClient) ObjectExists(ctx context.Context, bucket, object string) (bool, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		return false, errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return false, err
	}
	return bkt.IsObjectExist(object)
}
