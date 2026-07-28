// Package receiptstore 는 작은 S3 기반 저장소 경계를 보여준다.
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
	// ErrInvalidObject 는 안전하지 않거나 불완전한 객체 요청을 나타낸다.
	ErrInvalidObject = errors.New("receiptstore: invalid object")
	// ErrObjectNotFound 는 누락된 S3 객체를 애플리케이션 수준 오류로 나타낸다.
	ErrObjectNotFound = errors.New("receiptstore: object not found")
)

// Client 는 Store 에 필요한 좁은 S3 클라이언트 표면이다.
type Client interface {
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// Presigner 는 호출자 소유 다운로드 링크에 필요한 좁은 presign 표면이다.
type Presigner interface {
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// Store 는 버킷 이름, 객체 키 규칙, 메타데이터 계약을 소유한다.
type Store struct {
	Bucket      string
	DownloadTTL time.Duration
}

// ReceiptDocument 는 애플리케이션이 소유하는 업로드 명령이다.
type ReceiptDocument struct {
	TenantID  string            `json:"tenant_id"`
	ReceiptID string            `json:"receipt_id"`
	FileName  string            `json:"file_name"`
	Body      []byte            `json:"body,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// StoredObject 는 S3에 저장된 객체를 설명한다.
type StoredObject struct {
	TenantID    string            `json:"tenant_id"`
	ReceiptID   string            `json:"receipt_id"`
	FileName    string            `json:"file_name"`
	Key         string            `json:"key"`
	ContentType string            `json:"content_type"`
	Size        int64             `json:"size"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// DownloadedObject 는 S3 객체 본문과 선택된 메타데이터를 담는다.
type DownloadedObject struct {
	StoredObject
	Body []byte `json:"body,omitempty"`
}

// PresignedDownload 는 호출자에게 반환되는 presigned URL 결과를 설명한다.
type PresignedDownload struct {
	Method    string        `json:"method"`
	URL       string        `json:"url"`
	ExpiresIn time.Duration `json:"expires_in"`
}

// Preview 는 S3에 접속하지 않고 예제 계약을 설명한다.
type Preview struct {
	Bucket        string   `json:"bucket"`
	ObjectPrefix  string   `json:"object_prefix"`
	Operations    []string `json:"operations"`
	LocalContract []string `json:"local_contract"`
	SmokeTest     string   `json:"smoke_test"`
}

// NewStore 는 S3 영수증 저장소를 생성한다.
func NewStore(bucket string, downloadTTL time.Duration) (Store, error) {
	if bucket == "" {
		return Store{}, fmt.Errorf("%w: bucket is required", ErrInvalidObject)
	}
	if downloadTTL <= 0 {
		downloadTTL = defaultDownloadTTL
	}
	return Store{Bucket: bucket, DownloadTTL: downloadTTL}, nil
}

// NewPreview 는 README 와 go run 출력용 로컬 미리보기를 구성한다.
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

// Upload 는 tenant 영수증 객체를 S3에 저장한다.
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

// Download 는 영수증 객체를 읽고 응답 본문을 닫는다.
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

// ListTenantReceipts 는 하나의 tenant prefix 아래 영수증 객체를 나열한다.
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

// Delete 는 영수증 객체 하나를 제거한다.
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

// PresignDownload 는 영수증 객체 하나에 대한 호출자용 GET URL을 생성한다.
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

// SampleDocuments 는 미리보기와 테스트에 사용할 tenant 형태 데이터를 반환한다.
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
