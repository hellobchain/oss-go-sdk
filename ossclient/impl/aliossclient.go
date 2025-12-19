package impl

import (
	"bytes"
	"context"
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
	logger ossclient.Logger
}

// SetLogger implements ossclient.OssClient.
func (c *aliClient) SetLogger(logger ossclient.Logger) {
	c.logger = logger
}

// SetBucket implements ossclient.OssClient.
func (c *aliClient) SetBucket(bucket string) {
	c.bucket = bucket
}

func NewAliClient(clientConfig *models.Config) (ossclient.OssClient, error) {
	aliClient := &aliClient{logger: &ossclient.DefaultLogger{}}
	if clientConfig.Endpoint == "" || clientConfig.AccessKeyID == "" || clientConfig.SecretAccessKey == "" {
		aliClient.logger.Print("invalid configuration")
		return nil, errors.ErrInvalidConfig
	}
	_, err := url.Parse(clientConfig.Endpoint)
	if err != nil {
		aliClient.logger.Print("invalid endpoint", clientConfig.Endpoint)
		return nil, err
	}
	cli, err := oss.New(clientConfig.Endpoint, clientConfig.AccessKeyID, clientConfig.SecretAccessKey, oss.InsecureSkipVerify(true))
	if err != nil {
		aliClient.logger.Print("failed to create oss client", err)
		return nil, err
	}
	if clientConfig.Region == "" {
		cli.SetRegion(clientConfig.Region)
	}
	aliClient.bucket = clientConfig.BucketName
	aliClient.cli = cli
	if clientConfig.BucketName != "" {
		err = aliClient.EnsureBucketExists(context.Background(), clientConfig.BucketName)
		if err != nil {
			aliClient.logger.Print("failed to create ali client", err)
			// return nil, err
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
		c.logger.Print("failed to get bucket", err)
		return err
	}
	return bkt.PutObject(object, bytes.NewReader(data))
}

func (c *aliClient) UploadFile(ctx context.Context, bucket, object string, filePath string, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		c.logger.Print("client not initialized")
		return errors.ErrClientNotInitialized
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.logger.Print("file does not exist", err)
		return err
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("bucket does not exist", err)
		return err
	}
	return bkt.PutObjectFromFile(object, filePath)
}
func (c *aliClient) UploadFromReader(ctx context.Context, bucket, object string, reader io.Reader, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		c.logger.Print("oss client not initialized")
		return errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("bucket does not exist", err)
		return err
	}
	return bkt.PutObject(object, reader)
}

func (c *aliClient) Download(ctx context.Context, bucket, object string) ([]byte, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		c.logger.Print("oss client not initialized")
		return nil, errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("bucket does not exist", err)
		return nil, err
	}
	out, err := bkt.GetObject(object)
	if err != nil {
		c.logger.Print("object does not exist", err)
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
		c.logger.Print("client not initialized")
		return errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("bucket does not exist", err)
		return err
	}
	return bkt.GetObjectToFile(object, filePath)
}

func (c *aliClient) DownloadTo(ctx context.Context, bucket, object string, w io.Writer) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		c.logger.Print("client not initialized")
		return errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("bucket does not exist", err)
		return err
	}
	rc, err := bkt.GetObject(object)
	if err != nil {
		c.logger.Print("object does not exist", err)
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
		c.logger.Print("client not initialized")
		return nil, errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("bucket does not exist", err)
		return nil, err
	}
	marker := ""
	var keys []string
	for {
		lr, err := bkt.ListObjects(oss.Prefix(prefix), oss.Marker(marker))
		if err != nil {
			c.logger.Print("list objects failed", err)
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
		c.logger.Print("ali client not initialized")
		return errors.ErrClientNotInitialized
	}
	exist, err := c.cli.IsBucketExist(bucket)
	if err != nil {
		c.logger.Print("check bucket exist failed", err)
		return err
	}
	if exist {
		return nil
	}
	// 创建，ACL 默认私有
	err = c.cli.CreateBucket(bucket, oss.ACL(oss.ACLPrivate))
	if err != nil {
		c.logger.Print("create bucket failed", err)
		return err
	}
	return nil
}

func (c *aliClient) GetObjectInfo(ctx context.Context, bucket, object string) (*models.ObjectInfo, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		c.logger.Print("ali client not initialized")
		return nil, errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("get bucket failed", err)
		return nil, err
	}
	meta, err := bkt.GetObjectDetailedMeta(object)
	if err != nil {
		c.logger.Print("get object meta failed", err)
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
		c.logger.Print("oss client not initialized")
		return errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("get bucket handle", err)
		return err
	}
	err = bkt.DeleteObject(object)
	if err != nil {
		c.logger.Print("delete object", err)
		return err
	}
	return nil
}

func (c *aliClient) ObjectExists(ctx context.Context, bucket, object string) (bool, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.cli == nil {
		c.logger.Print("client not initialized")
		return false, errors.ErrClientNotInitialized
	}
	bkt, err := c.cli.Bucket(bucket)
	if err != nil {
		c.logger.Print("get bucket handle", err)
		return false, err
	}
	boolValue, err := bkt.IsObjectExist(object)
	if err != nil {
		c.logger.Print("failed to check object exists", object)
		return false, err
	}
	return boolValue, nil

}
