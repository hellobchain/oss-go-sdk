package impl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
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

func NewAliClient(accessKey, secretKey, endpoint, region, bucket string) (ossclient.OssClient, error) {
	cli, err := oss.New(endpoint, accessKey, secretKey)
	if err != nil {
		return nil, err
	}
	if region == "" {
		cli.SetRegion(region)
	}
	aliClient := &aliClient{cli: cli, bucket: bucket}
	if bucket != "" {
		if aliClient.EnsureBucketExists(context.Background(), bucket) != nil {
			return nil, err
		}
	}
	return aliClient, nil
}

func (c *aliClient) Upload(ctx context.Context, bucket, object string, data []byte, _ ...ossclient.UploadOpt) error {
	if bucket == "" {
		bucket = c.bucket
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return err
	}
	return bkt.PutObject(object, bytes.NewReader(data))
}

func (c *aliClient) UploadFile(ctx context.Context, bucket, object string, filePath string, _ ...ossclient.UploadOpt) error {
	if bucket == "" {
		bucket = c.bucket
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
func (c *aliClient) UploadFromReader(ctx context.Context, bucket, object string, reader io.Reader, _ ...ossclient.UploadOpt) error {
	if bucket == "" {
		bucket = c.bucket
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

func (c *aliClient) GetObjectInfo(ctx context.Context, bucket, object string) (*ossclient.ObjectInfo, error) {
	if bucket == "" {
		bucket = c.bucket
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
	modTime, _ := http.ParseTime(lm)
	return &ossclient.ObjectInfo{
		Key:          object,
		Size:         sz,
		LastModified: modTime,
		ETag:         strings.Trim(etag, `"`),
	}, nil
}

func (c *aliClient) DeleteObject(ctx context.Context, bucket, object string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		return err
	}
	return bkt.DeleteObject(object)
}
