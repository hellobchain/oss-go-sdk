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
	logger ossclient.Logger
}

// SetLogger implements ossclient.OssClient.
func (c *s3Client) SetLogger(logger ossclient.Logger) {
	c.logger = logger
}

// SetBucket implements ossclient.OssClient.
func (c *s3Client) SetBucket(bucket string) {
	c.bucket = bucket
}

func NewS3Client(clientConfig *models.Config) (ossclient.OssClient, error) {
	s3Client := &s3Client{logger: &ossclient.DefaultLogger{}}
	if clientConfig.Endpoint == "" || clientConfig.AccessKeyID == "" || clientConfig.SecretAccessKey == "" {
		s3Client.logger.Print("endpoint:", clientConfig.Endpoint)
		return nil, errors.ErrInvalidConfig
	}
	// 自动识别 PathStyle
	if clientConfig.Region == "" {
		clientConfig.Region = "us-east-1"
	}

	u, err := url.Parse(clientConfig.Endpoint)
	if err != nil {
		s3Client.logger.Print("invalid endpoint", clientConfig.Endpoint)
		return nil, err
	}
	sessionConfig := &aws.Config{
		Region:           aws.String(clientConfig.Region),
		Endpoint:         aws.String(clientConfig.Endpoint),
		Credentials:      credentials.NewStaticCredentials(clientConfig.AccessKeyID, clientConfig.SecretAccessKey, ""),
		S3ForcePathStyle: aws.Bool(!clientConfig.IsS3),
		DisableSSL:       aws.Bool(u.Scheme == "http"),
	}
	sess, err := session.NewSession(sessionConfig)
	if err != nil {
		s3Client.logger.Print("Error creating session: %v", err)
		return nil, err
	}
	svc := s3.New(sess)
	s3Client.bucket = clientConfig.BucketName
	s3Client.svc = svc
	if clientConfig.BucketName != "" {
		err = s3Client.EnsureBucketExists(context.Background(), clientConfig.BucketName)
		if err != nil {
			s3Client.logger.Print("failed to ensure bucket exists", err)
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
		c.logger.Print("invalid configuration")
		return errors.ErrClientNotInitialized
	}
	_, err := c.svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		c.logger.Print("failed to upload file", err)
		return errors.ErrUploadFailed
	}
	return nil
}

// DownloadFile implements ossclient.OssClient.
func (c *s3Client) DownloadFile(ctx context.Context, bucket string, object string, filePath string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		c.logger.Print("client not initialized")
		return errors.ErrClientNotInitialized
	}
	out, err := c.svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		c.logger.Print("GetObjectWithContext failed")
		return err
	}
	defer out.Body.Close()
	fd, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(0664))
	if err != nil {
		c.logger.Print("failed to open file", filePath)
		return err
	}
	// Copy the data to the local file path.
	_, err = io.Copy(fd, out.Body)
	fd.Close()
	if err != nil {
		c.logger.Print("failed to write file", filePath)
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
		c.logger.Print("client not initialized")
		return errors.ErrClientNotInitialized
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.logger.Print("file does not exist", err)
		return err
	}
	fd, err := os.Open(filePath)
	if err != nil {
		c.logger.Print("file open failed", err)
		return err
	}
	defer fd.Close()
	_, err = c.svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   aws.ReadSeekCloser(fd),
	})
	if err != nil {
		c.logger.Print("file upload failed", err)
		return errors.ErrUploadFailed
	}
	return nil
}

// UploadFromReader implements ossclient.OssClient.
func (c *s3Client) UploadFromReader(ctx context.Context, bucket string, object string, reader io.Reader, _ ...models.UploadOptions) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		c.logger.Print("oss client not initialized")
		return errors.ErrClientNotInitialized
	}
	_, err := c.svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Body:   aws.ReadSeekCloser(reader),
	})
	if err != nil {
		c.logger.Print("upload failed", err)
		return errors.ErrUploadFailed
	}
	return nil
}
func (c *s3Client) Download(ctx context.Context, bucket, object string) ([]byte, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		c.logger.Print("oss client not initialized")
		return nil, errors.ErrClientNotInitialized
	}
	out, err := c.svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		c.logger.Print("get object failed", err)
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
		c.logger.Print("client not initialized")
		return errors.ErrClientNotInitialized
	}
	out, err := c.svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		c.logger.Print("get object failed", err)
		return err
	}
	defer out.Body.Close()
	_, err = io.Copy(w, out.Body)
	if err != nil {
		c.logger.Print("failed to write file", err)
		return err
	}
	return nil
}
func (c *s3Client) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		c.logger.Print("client not initialized")
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
	if err != nil {
		c.logger.Print("list objects failed", err)
		return nil, err
	}
	return keys, nil
}

func (c *s3Client) EnsureBucketExists(ctx context.Context, bucket string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		c.logger.Print("s3 client not initialized")
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
	if err != nil {
		c.logger.Print("create bucket: ", err)
		return err
	}
	return nil
}

func (c *s3Client) GetObjectInfo(ctx context.Context, bucket, object string) (*models.ObjectInfo, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		c.logger.Print("s3 client not initialized")
		return nil, errors.ErrClientNotInitialized
	}
	head, err := c.svc.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		c.logger.Print(err)
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
		c.logger.Print("oss client not initialized")
		return errors.ErrClientNotInitialized
	}
	_, err := c.svc.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		c.logger.Print(err)
		return errors.ErrObjectNotExists
	}
	return nil
}

func (c *s3Client) ObjectExists(ctx context.Context, bucket, object string) (bool, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	if c.svc == nil {
		c.logger.Print("client not initialized")
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
		c.logger.Print(err.Error())
		return false, err
	}
	return true, nil
}
