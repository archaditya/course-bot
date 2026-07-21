// Package r2 implements provider.ObjectStore against Cloudflare R2 using the
// AWS SDK v2 (R2 is S3-compatible). See docs/07-storage.md and
// docs/08-security.md#r2-signed-urls. The frontend never calls R2 directly —
// all access goes through short-lived signed URLs issued by the Go API.
package r2

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"course-assistant/internal/domain/provider"
)

// Store wraps an S3 client pointed at Cloudflare R2.
type Store struct {
	client *s3.Client
	signer *s3.PresignClient
	bucket string
}

// NewStore creates a Store using the R2 S3-compatible endpoint.
// accountID must be the Cloudflare account ID; credentials are the R2 API token
// pair (not the Cloudflare global API key). See docs/08-security.md#secrets.
func NewStore(accountID, accessKeyID, secretAccessKey, bucket string) (*Store, error) {
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID, secretAccessKey, "",
		)),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("r2: load config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // R2 requires path-style
	})

	return &Store{
		client: client,
		signer: s3.NewPresignClient(client),
		bucket: bucket,
	}, nil
}

// Put stores data at key with the given content type. Raw uploads go to
// "raw/<key>"; normalized docs to "processed/<key>" — the key conventions
// are the caller's responsibility. See docs/07-storage.md#layout.
func (s *Store) Put(ctx context.Context, key string, data []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("r2: put %s: %w", key, err)
	}
	return nil
}

// Get retrieves the object at key. Returns the raw bytes.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("r2: get %s: %w", key, err)
	}
	defer out.Body.Close()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("r2: read %s: %w", key, err)
	}
	return data, nil
}

// SignedPutURL issues a short-lived, single-use presigned PUT URL scoped to
// one object key. The browser uploads directly to R2 using this URL —
// the Go API never proxies the file bytes, per docs/08-security.md#r2-signed-urls.
func (s *Store) SignedPutURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.signer.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(o *s3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("r2: sign put url %s: %w", key, err)
	}
	return req.URL, nil
}

// SignedGetURL issues a short-lived presigned GET URL for serving a file
// (e.g. video playback). Reissued per session as needed.
func (s *Store) SignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.signer.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(o *s3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("r2: sign get url %s: %w", key, err)
	}
	return req.URL, nil
}

// Compile-time assertion: Store implements the domain interface.
var _ provider.ObjectStore = (*Store)(nil)
