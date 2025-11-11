package impl

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/hellobchain/oss-go-sdk/common/errors"
	"github.com/hellobchain/oss-go-sdk/common/models"
	"github.com/hellobchain/oss-go-sdk/ossclient"
)

/*---------- MinIO / 兼容S3实现 ----------*/
type s3Client struct {
	svc    *s3.S3
	bucket string
}

// SetBucket implements ossclient.OssClient.
func (c *s3Client) SetBucket(bucket string) {
	c.bucket = bucket
}

func NewS3Client(clientConfig *models.Config) (ossclient.OssClient, error) {
	if clientConfig.Endpoint == "" || clientConfig.AccessKeyID == "" || clientConfig.SecretAccessKey == "" {
		return nil, errors.ErrInvalidConfig
	}
	// 自动识别 PathStyle
	if clientConfig.Region == "" {
		clientConfig.Region = "us-east-1"
	}

	u, err := url.Parse(clientConfig.Endpoint)
	if err != nil {
		return nil, err
	}
	pathStyle := strings.Contains(u.Host, "localhost") || strings.Contains(u.Host, "127.0.0.1")
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(clientConfig.Region),
		Endpoint:         aws.String(clientConfig.Endpoint),
		Credentials:      credentials.NewStaticCredentials(clientConfig.AccessKeyID, clientConfig.SecretAccessKey, ""),
		S3ForcePathStyle: aws.Bool(pathStyle),
		DisableSSL:       aws.Bool(u.Scheme == "http"),
	})
	if err != nil {
		return nil, err
	}
	svc := s3.New(sess)
	s3Client := &s3Client{svc: svc, bucket: clientConfig.BucketName}
	if clientConfig.BucketName != "" {
		err = s3Client.EnsureBucketExists(context.Background(), clientConfig.BucketName)
		if err != nil {
			return nil, err
		}
	}
	return s3Client, nil
}

func (c *s3Client) Upload(ctx context.Context, bucket, object string, data []byte, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return errors.ErrClientNotInitialized
	}
	_, err := c.svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   bytes.NewReader(data),
	})
	return err
}

// DownloadFile implements ossclient.OssClient.
func (c *s3Client) DownloadFile(ctx context.Context, bucket string, object string, filePath string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return errors.ErrClientNotInitialized
	}
	out, err := c.svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()
	fd, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(0664))
	if err != nil {
		return err
	}
	// Copy the data to the local file path.
	_, err = io.Copy(fd, out.Body)
	fd.Close()
	if err != nil {
		return err
	}
	return nil
}

// UploadFile implements ossclient.OssClient.
func (c *s3Client) UploadFile(ctx context.Context, bucket string, object string, filePath string, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return errors.ErrClientNotInitialized
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}
	fd, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = c.svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   aws.ReadSeekCloser(fd),
	})
	return err
}

// UploadFromReader implements ossclient.OssClient.
func (c *s3Client) UploadFromReader(ctx context.Context, bucket string, object string, reader io.Reader, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return errors.ErrClientNotInitialized
	}
	_, err := c.svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   aws.ReadSeekCloser(reader),
	})
	return err
}
func (c *s3Client) Download(ctx context.Context, bucket, object string) ([]byte, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return nil, errors.ErrClientNotInitialized
	}
	out, err := c.svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}
func (c *s3Client) DownloadTo(ctx context.Context, bucket, object string, w io.Writer) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return errors.ErrClientNotInitialized
	}
	out, err := c.svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()
	_, err = io.Copy(w, out.Body)
	return err
}
func (c *s3Client) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return nil, errors.ErrClientNotInitialized
	}
	var keys []string
	err := c.svc.ListObjectsPagesWithContext(ctx, &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}, func(p *s3.ListObjectsOutput, last bool) bool {
		for _, o := range p.Contents {
			keys = append(keys, *o.Key)
		}
		return !last
	})
	return keys, err
}

func (c *s3Client) EnsureBucketExists(ctx context.Context, bucket string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return errors.ErrClientNotInitialized
	}
	// 先判断
	_, err := c.svc.HeadBucketWithContext(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}
	// 404 则创建
	_, err = c.svc.CreateBucketWithContext(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	return err
}

func (c *s3Client) GetObjectInfo(ctx context.Context, bucket, object string) (*models.ObjectInfo, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return nil, errors.ErrClientNotInitialized
	}
	head, err := c.svc.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return nil, err
	}
	var size int64
	if head.ContentLength != nil {
		size = *head.ContentLength
	}
	var lastModified string
	if head.LastModified != nil {
		lastModified = (*head.LastModified).Format(time.DateTime)
	}
	var contentType string
	if head.ContentType != nil {
		contentType = *head.ContentType
	}
	userMetadata := make(map[string]string)
	if head.Metadata != nil {
		for k, v := range head.Metadata {
			if v == nil {
				continue
			}
			userMetadata[k] = *v
		}
	}
	var etag string
	if head.ETag != nil {
		etag = strings.Trim(*head.ETag, `"`)
	}
	return &models.ObjectInfo{
		Key:          object,
		Size:         size,
		LastModified: lastModified,
		ETag:         etag,
		ContentType:  contentType,
		UserMetadata: userMetadata,
	}, nil
}

func (c *s3Client) DeleteObject(ctx context.Context, bucket, object string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return errors.ErrClientNotInitialized
	}
	_, err := c.svc.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	return err
}

func (c *s3Client) ObjectExists(ctx context.Context, bucket, object string) (bool, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		return false, errors.ErrClientNotInitialized
	}
	_, err := c.svc.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		// AWS 返回 404 表示不存在
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
