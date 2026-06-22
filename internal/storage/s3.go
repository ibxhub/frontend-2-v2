package storage

import (
	"context"
	"errors"
	"io"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type S3 struct {
	bucket     string
	client     *s3.Client
	uploader   *manager.Uploader
	disableKMS bool
}

func NewS3(ctx context.Context, region, bucket string) (*S3, error) {
	return NewS3WithEndpoint(ctx, region, bucket, "")
}

func NewS3WithEndpoint(ctx context.Context, region, bucket, endpoint string) (*S3, error) {
	if bucket == "" {
		return nil, errors.New("bucket is required")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
			o.EndpointOptions.DisableHTTPS = true
		}
	})
	return &S3{
		bucket:   bucket,
		client:   client,
		uploader: manager.NewUploader(client),
	}, nil
}

func (s *S3) Put(ctx context.Context, name string, r io.Reader, _ int64) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
		Body:   r,
	}
	if !s.disableKMS {
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
	}
	_, err := s.uploader.Upload(ctx, input)
	return err
}

func (s *S3) Get(ctx context.Context, name string) (io.ReadCloser, *Object, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(name),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
			return nil, nil, ErrNotFound
		}
		return nil, nil, err
	}
	obj := &Object{Name: name}
	if out.ContentLength != nil {
		obj.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		obj.ModTime = *out.LastModified
	}
	return out.Body, obj, nil
}

func (s *S3) List(ctx context.Context) ([]Object, error) {
	var objects []Object
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, item := range page.Contents {
			obj := Object{}
			if item.Key != nil {
				obj.Name = *item.Key
			}
			if item.Size != nil {
				obj.Size = *item.Size
			}
			if item.LastModified != nil {
				obj.ModTime = *item.LastModified
			}
			objects = append(objects, obj)
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].ModTime.After(objects[j].ModTime) })
	return objects, nil
}
