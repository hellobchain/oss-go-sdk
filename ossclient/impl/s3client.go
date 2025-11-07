package impl

import (
	"bytes"
	"context"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
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

func NewS3Client(accessKey, secretKey, endpoint, region, bucket string) (ossclient.OssClient, error) {
	// 自动识别 PathStyle
	u, _ := url.Parse(endpoint)
	pathStyle := strings.Contains(u.Host, "localhost") || strings.Contains(u.Host, "127.0.0.1")
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String(region),
		Endpoint:         aws.String(endpoint),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		S3ForcePathStyle: aws.Bool(pathStyle),
		DisableSSL:       aws.Bool(u.Scheme == "http"),
	})
	if err != nil {
		return nil, err
	}
	svc := s3.New(sess)
	s3Client := &s3Client{svc: svc, bucket: bucket}
	if bucket != "" {
		if s3Client.EnsureBucketExists(context.Background(), bucket) != nil {
			return nil, err
		}
	}
	return s3Client, nil
}

func (c *s3Client) Upload(ctx context.Context, bucket, object string, data []byte, _ ...ossclient.UploadOpt) error {
	if bucket == "" {
		bucket = c.bucket
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
func (c *s3Client) UploadFile(ctx context.Context, bucket string, object string, filePath string, _ ...ossclient.UploadOpt) error {
	if bucket == "" {
		bucket = c.bucket
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
func (c *s3Client) UploadFromReader(ctx context.Context, bucket string, object string, reader io.Reader, _ ...ossclient.UploadOpt) error {
	if bucket == "" {
		bucket = c.bucket
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

func (c *s3Client) GetObjectInfo(ctx context.Context, bucket, object string) (*ossclient.ObjectInfo, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	head, err := c.svc.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	if err != nil {
		return nil, err
	}
	return &ossclient.ObjectInfo{
		Key:          object,
		Size:         *head.ContentLength,
		LastModified: *head.LastModified,
		ETag:         strings.Trim(*head.ETag, `"`),
	}, nil
}

func (c *s3Client) DeleteObject(ctx context.Context, bucket, object string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	_, err := c.svc.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
	})
	return err
}
