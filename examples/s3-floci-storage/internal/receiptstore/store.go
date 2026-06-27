// Package receiptstore demonstrates a small S3-backed storage boundary.
package receiptstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const defaultDownloadTTL = 15 * time.Minute

var (
	// ErrInvalidObject reports an object request that is unsafe or incomplete.
	ErrInvalidObject = errors.New("receiptstore: invalid object")
	// ErrObjectNotFound reports a missing S3 object using an application-level error.
	ErrObjectNotFound = errors.New("receiptstore: object not found")
)

// Client is the narrow S3 client surface needed by Store.
type Client interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// Presigner is the narrow presign surface needed for caller-owned download links.
type Presigner interface {
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// Store owns the bucket name, object key convention, and metadata contract.
type Store struct {
	Bucket      string
	DownloadTTL time.Duration
}

// ReceiptDocument is the application-owned upload command.
type ReceiptDocument struct {
	TenantID  string            `json:"tenant_id"`
	ReceiptID string            `json:"receipt_id"`
	FileName  string            `json:"file_name"`
	Body      []byte            `json:"body,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// StoredObject describes the object persisted in S3.
type StoredObject struct {
	TenantID    string            `json:"tenant_id"`
	ReceiptID   string            `json:"receipt_id"`
	FileName    string            `json:"file_name"`
	Key         string            `json:"key"`
	ContentType string            `json:"content_type"`
	Size        int64             `json:"size"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// DownloadedObject contains the S3 object body and selected metadata.
type DownloadedObject struct {
	StoredObject
	Body []byte `json:"body,omitempty"`
}

// PresignedDownload describes the caller-facing presigned URL result.
type PresignedDownload struct {
	Method    string        `json:"method"`
	URL       string        `json:"url"`
	ExpiresIn time.Duration `json:"expires_in"`
}

// Preview describes the example contract without contacting S3.
type Preview struct {
	Bucket        string   `json:"bucket"`
	ObjectPrefix  string   `json:"object_prefix"`
	Operations    []string `json:"operations"`
	LocalContract []string `json:"local_contract"`
	SmokeTest     string   `json:"smoke_test"`
}

// NewStore creates an S3 receipt store.
func NewStore(bucket string, downloadTTL time.Duration) (Store, error) {
	if bucket == "" {
		return Store{}, fmt.Errorf("%w: bucket is required", ErrInvalidObject)
	}
	if downloadTTL <= 0 {
		downloadTTL = defaultDownloadTTL
	}
	return Store{Bucket: bucket, DownloadTTL: downloadTTL}, nil
}

// NewPreview builds a local preview for README and go run output.
func NewPreview(bucket string) (Preview, error) {
	if _, err := NewStore(bucket, defaultDownloadTTL); err != nil {
		return Preview{}, err
	}
	return Preview{
		Bucket:       bucket,
		ObjectPrefix: "tenants/<tenant_id>/receipts/<receipt_id>/<file_name>",
		Operations: []string{
			"Upload stores body, content type, size, and metadata",
			"Download reads and closes the S3 response body",
			"ListTenantReceipts uses a tenant prefix",
			"Delete removes one receipt object",
			"PresignDownload creates a caller-owned GET URL",
		},
		LocalContract: []string{
			"Floci S3 uses path-style addressing and endpoint override",
			"real AWS uses the same AWS SDK request types with normal credentials",
			"NoSuchKey becomes ErrObjectNotFound",
		},
		SmokeTest: "BLUETAPE_S3_FLOCI_STORAGE_SMOKE=1 go test -run TestFlociSmoke -count=1 ./examples/s3-floci-storage/...",
	}, nil
}

// Upload stores a tenant receipt object in S3.
func (s Store) Upload(ctx context.Context, client Client, doc ReceiptDocument) (StoredObject, error) {
	if err := ctx.Err(); err != nil {
		return StoredObject{}, err
	}
	if err := s.validate(); err != nil {
		return StoredObject{}, err
	}
	if err := validateDocument(doc); err != nil {
		return StoredObject{}, err
	}

	object := StoredObject{
		TenantID:    doc.TenantID,
		ReceiptID:   doc.ReceiptID,
		FileName:    path.Base(doc.FileName),
		Key:         objectKey(doc.TenantID, doc.ReceiptID, doc.FileName),
		ContentType: contentTypeForKey(doc.FileName, doc.Body),
		Size:        int64(len(doc.Body)),
		Metadata:    objectMetadata(doc),
	}
	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.Bucket),
		Key:           aws.String(object.Key),
		Body:          bytes.NewReader(doc.Body),
		ContentLength: aws.Int64(object.Size),
		ContentType:   aws.String(object.ContentType),
		Metadata:      object.Metadata,
	})
	if err != nil {
		return StoredObject{}, fmt.Errorf("upload receipt %s: %w", object.Key, err)
	}
	return object, nil
}

// Download reads a receipt object and closes the response body.
func (s Store) Download(ctx context.Context, client Client, tenantID, receiptID, fileName string) (DownloadedObject, error) {
	if err := ctx.Err(); err != nil {
		return DownloadedObject{}, err
	}
	if err := s.validate(); err != nil {
		return DownloadedObject{}, err
	}
	key, err := checkedObjectKey(tenantID, receiptID, fileName)
	if err != nil {
		return DownloadedObject{}, err
	}

	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isS3NotFound(err) {
			return DownloadedObject{}, fmt.Errorf("%w: %s", ErrObjectNotFound, key)
		}
		return DownloadedObject{}, fmt.Errorf("download receipt %s: %w", key, err)
	}
	if output.Body == nil {
		return DownloadedObject{}, fmt.Errorf("download receipt %s: %w", key, ErrObjectNotFound)
	}
	defer func() {
		_ = output.Body.Close()
	}()
	payload, err := io.ReadAll(output.Body)
	if err != nil {
		return DownloadedObject{}, fmt.Errorf("read receipt %s: %w", key, err)
	}

	return DownloadedObject{
		StoredObject: StoredObject{
			TenantID:    tenantID,
			ReceiptID:   receiptID,
			FileName:    path.Base(fileName),
			Key:         key,
			ContentType: aws.ToString(output.ContentType),
			Size:        int64(len(payload)),
			Metadata:    output.Metadata,
		},
		Body: payload,
	}, nil
}

// ListTenantReceipts lists receipt objects under one tenant prefix.
func (s Store) ListTenantReceipts(ctx context.Context, client Client, tenantID string) ([]StoredObject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	if err := validateSegment("tenant_id", tenantID); err != nil {
		return nil, err
	}
	prefix := tenantPrefix(tenantID)
	output, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("list tenant receipts %s: %w", tenantID, err)
	}
	items := make([]StoredObject, 0, len(output.Contents))
	for _, item := range output.Contents {
		key := aws.ToString(item.Key)
		receiptID, fileName := parseReceiptKey(tenantID, key)
		items = append(items, StoredObject{
			TenantID:  tenantID,
			ReceiptID: receiptID,
			FileName:  fileName,
			Key:       key,
			Size:      aws.ToInt64(item.Size),
		})
	}
	return items, nil
}

// Delete removes one receipt object.
func (s Store) Delete(ctx context.Context, client Client, tenantID, receiptID, fileName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.validate(); err != nil {
		return err
	}
	key, err := checkedObjectKey(tenantID, receiptID, fileName)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete receipt %s: %w", key, err)
	}
	return nil
}

// PresignDownload creates a caller-facing GET URL for one receipt object.
func (s Store) PresignDownload(ctx context.Context, presigner Presigner, tenantID, receiptID, fileName string) (PresignedDownload, error) {
	if err := ctx.Err(); err != nil {
		return PresignedDownload{}, err
	}
	if err := s.validate(); err != nil {
		return PresignedDownload{}, err
	}
	key, err := checkedObjectKey(tenantID, receiptID, fileName)
	if err != nil {
		return PresignedDownload{}, err
	}
	expires := s.DownloadTTL
	request, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	}, func(options *s3.PresignOptions) {
		options.Expires = expires
	})
	if err != nil {
		return PresignedDownload{}, fmt.Errorf("presign receipt %s: %w", key, err)
	}
	return PresignedDownload{Method: request.Method, URL: request.URL, ExpiresIn: expires}, nil
}

// SampleDocuments returns tenant-shaped data for preview and tests.
func SampleDocuments() []ReceiptDocument {
	return []ReceiptDocument{
		{TenantID: "tenant-alpha", ReceiptID: "receipt-1001", FileName: "invoice-1001.txt", Body: []byte("receipt 1001\n"), Metadata: map[string]string{"source": "checkout"}},
		{TenantID: "tenant-alpha", ReceiptID: "receipt-1002", FileName: "invoice-1002.txt", Body: []byte("receipt 1002\n"), Metadata: map[string]string{"source": "checkout"}},
		{TenantID: "tenant-beta", ReceiptID: "receipt-2001", FileName: "invoice-2001.txt", Body: []byte("receipt 2001\n"), Metadata: map[string]string{"source": "batch"}},
	}
}

func (s Store) validate() error {
	if s.Bucket == "" {
		return fmt.Errorf("%w: bucket is required", ErrInvalidObject)
	}
	return nil
}

func validateDocument(doc ReceiptDocument) error {
	if err := validateSegment("tenant_id", doc.TenantID); err != nil {
		return err
	}
	if err := validateSegment("receipt_id", doc.ReceiptID); err != nil {
		return err
	}
	if err := validateFileName(doc.FileName); err != nil {
		return err
	}
	if len(doc.Body) == 0 {
		return fmt.Errorf("%w: body is required", ErrInvalidObject)
	}
	return nil
}

func checkedObjectKey(tenantID, receiptID, fileName string) (string, error) {
	if err := validateSegment("tenant_id", tenantID); err != nil {
		return "", err
	}
	if err := validateSegment("receipt_id", receiptID); err != nil {
		return "", err
	}
	if err := validateFileName(fileName); err != nil {
		return "", err
	}
	return objectKey(tenantID, receiptID, fileName), nil
}

func validateSegment(name, value string) error {
	if value == "" || strings.Contains(value, "/") || strings.Contains(value, "..") {
		return fmt.Errorf("%w: %s must be a non-empty safe key segment", ErrInvalidObject, name)
	}
	return nil
}

func validateFileName(fileName string) error {
	if fileName == "" || path.Base(fileName) != fileName || strings.Contains(fileName, "..") {
		return fmt.Errorf("%w: file_name must be a safe base name", ErrInvalidObject)
	}
	return nil
}

func objectKey(tenantID, receiptID, fileName string) string {
	return path.Join(tenantPrefix(tenantID), receiptID, path.Base(fileName))
}

func tenantPrefix(tenantID string) string {
	return path.Join("tenants", tenantID, "receipts") + "/"
}

func parseReceiptKey(tenantID, key string) (string, string) {
	prefix := tenantPrefix(tenantID)
	rest := strings.TrimPrefix(key, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 {
		return "", path.Base(key)
	}
	return parts[0], parts[1]
}

func objectMetadata(doc ReceiptDocument) map[string]string {
	metadata := make(map[string]string, len(doc.Metadata)+2)
	for key, value := range doc.Metadata {
		metadata[key] = value
	}
	metadata["tenant_id"] = doc.TenantID
	metadata["receipt_id"] = doc.ReceiptID
	return metadata
}

func contentTypeForKey(key string, sample []byte) string {
	if typ := mime.TypeByExtension(filepath.Ext(key)); typ != "" {
		return typ
	}
	if len(sample) == 0 {
		return "application/octet-stream"
	}
	if len(sample) > 512 {
		sample = sample[:512]
	}
	return http.DetectContentType(sample)
}

func isS3NotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr interface{ ErrorCode() string }
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey"
}
