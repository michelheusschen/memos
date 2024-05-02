package s3

import (
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"

	storepb "github.com/usememos/memos/proto/gen/store"
)

const presignLifetimeSecs = 7 * 24 * 60 * 60

type Client struct {
	Client *s3.Client
	Bucket *string
}

func NewClient(ctx context.Context, s3Config *storepb.WorkspaceStorageSetting_S3Config) (*Client, error) {
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...any) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: s3Config.Endpoint,
		}, nil
	})
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3Config.AccessKeyId, s3Config.AccessKeySecret, "")),
		config.WithRegion(s3Config.Region),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load s3 config")
	}

	client := s3.NewFromConfig(cfg)
	return &Client{
		Client: client,
		Bucket: aws.String(s3Config.Bucket),
	}, nil
}

// UploadObject uploads an object to S3.
func (c *Client) UploadObject(ctx context.Context, key string, fileType string, content io.Reader) (string, error) {
	uploader := manager.NewUploader(c.Client)
	putInput := s3.PutObjectInput{
		Bucket:      c.Bucket,
		Key:         aws.String(key),
		ContentType: aws.String(fileType),
		Body:        content,
	}
	result, err := uploader.Upload(ctx, &putInput)
	if err != nil {
		return "", err
	}

	resultKey := result.Key
	if resultKey == nil || *resultKey == "" {
		return "", errors.New("failed to get file key")
	}
	return *resultKey, nil
}

// PresignGetObject presigns an object in S3.
func (c *Client) PresignGetObject(ctx context.Context, key string) (string, error) {
	presignClient := s3.NewPresignClient(c.Client)
	presignResult, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(*c.Bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(presignLifetimeSecs * int64(time.Second))
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to presign put object")
	}
	return presignResult.URL, nil
}

// DeleteObject deletes an object in S3.
func (c *Client) DeleteObject(ctx context.Context, key string) error {
	_, err := c.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: c.Bucket,
		Key:    aws.String(key),
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete object")
	}
	return nil
}
