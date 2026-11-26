package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type R2 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
}

func NewR2(ctx context.Context, cfg R2Config) (*R2, error) {
	if cfg.AccountID == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.Bucket == "" {
		return nil, fmt.Errorf("r2 config incomplete")
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		// R2 checksum behavior differs from S3 defaults; request checksums can break
		// presigned client uploads. We verify integrity with HEAD size checks.
		config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		config.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	return &R2{client: client, presign: s3.NewPresignClient(client), bucket: cfg.Bucket}, nil
}

func (r *R2) InitiateMultipart(ctx context.Context, key, contentType string) (string, error) {
	out, err := r.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", err
	}
	if out.UploadId == nil {
		return "", fmt.Errorf("missing upload id")
	}
	return *out.UploadId, nil
}

func (r *R2) PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int32, expires time.Duration) (string, error) {
	out, err := r.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(r.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (r *R2) ListParts(ctx context.Context, key, uploadID string) ([]UploadedPart, error) {
	out, err := r.client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(r.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		return nil, err
	}
	parts := make([]UploadedPart, 0, len(out.Parts))
	for _, p := range out.Parts {
		if p.PartNumber == nil || p.ETag == nil {
			continue
		}
		parts = append(parts, UploadedPart{PartNumber: *p.PartNumber, ETag: *p.ETag})
	}
	return parts, nil
}

func (r *R2) CompleteMultipart(ctx context.Context, key, uploadID string, parts []UploadedPart) error {
	completed := make([]types.CompletedPart, 0, len(parts))
	for _, p := range parts {
		pn := p.PartNumber
		etag := p.ETag
		completed = append(completed, types.CompletedPart{PartNumber: aws.Int32(pn), ETag: aws.String(etag)})
	}
	_, err := r.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(r.bucket),
		Key:             aws.String(key),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	return err
}

func (r *R2) AbortMultipart(ctx context.Context, key, uploadID string) error {
	_, err := r.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(r.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	return err
}

func (r *R2) Head(ctx context.Context, key string) (ObjectInfo, error) {
	out, err := r.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	var size int64
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return ObjectInfo{Size: size}, nil
}

func (r *R2) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (r *R2) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(r.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	_, err := r.client.PutObject(ctx, input)
	return err
}

func (r *R2) Delete(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (r *R2) PresignGet(ctx context.Context, key string, expires time.Duration, contentDisposition string) (string, error) {
	input := &s3.GetObjectInput{Bucket: aws.String(r.bucket), Key: aws.String(key)}
	if contentDisposition != "" {
		input.ResponseContentDisposition = aws.String(contentDisposition)
	}
	out, err := r.presign.PresignGetObject(ctx, input, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
