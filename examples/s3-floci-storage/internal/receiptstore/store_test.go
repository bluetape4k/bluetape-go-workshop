package receiptstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestPreviewShowsStorageBoundary(t *testing.T) {
	preview, err := NewPreview("tenant-receipts")
	if err != nil {
		t.Fatalf("NewPreview() error = %v", err)
	}
	if preview.ObjectPrefix != "tenants/<tenant_id>/receipts/<receipt_id>/<file_name>" {
		t.Fatalf("ObjectPrefix = %q", preview.ObjectPrefix)
	}
	if len(preview.Operations) != 5 || len(preview.LocalContract) != 3 {
		t.Fatalf("preview = %#v, want operations and local contract", preview)
	}
}

func TestStoreUploadMapsObjectKeyMetadataAndContentType(t *testing.T) {
	client := &fakeS3Client{}
	store := newTestStore(t)
	doc := SampleDocuments()[0]

	object, err := store.Upload(context.Background(), client, doc)
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}
	if object.Key != "tenants/tenant-alpha/receipts/receipt-1001/invoice-1001.txt" {
		t.Fatalf("Key = %q", object.Key)
	}
	if object.ContentType != "text/plain; charset=utf-8" {
		t.Fatalf("ContentType = %q", object.ContentType)
	}
	if client.putInput == nil {
		t.Fatal("PutObject input = nil")
	}
	if got := aws.ToString(client.putInput.Bucket); got != "tenant-receipts" {
		t.Fatalf("Bucket = %q", got)
	}
	if got := aws.ToString(client.putInput.Key); got != object.Key {
		t.Fatalf("Key = %q, want %q", got, object.Key)
	}
	if got := client.putInput.Metadata["tenant_id"]; got != "tenant-alpha" {
		t.Fatalf("metadata tenant_id = %q", got)
	}
	if got := aws.ToInt64(client.putInput.ContentLength); got != int64(len(doc.Body)) {
		t.Fatalf("ContentLength = %d", got)
	}
}

func TestStoreDownloadReadsAndClosesBody(t *testing.T) {
	body := &trackingReadCloser{Reader: bytes.NewReader([]byte("receipt 1001\n"))}
	client := &fakeS3Client{getOutput: &s3.GetObjectOutput{
		Body:        body,
		ContentType: aws.String("text/plain; charset=utf-8"),
		Metadata: map[string]string{
			"tenant_id":  "tenant-alpha",
			"receipt_id": "receipt-1001",
		},
	}}
	store := newTestStore(t)

	got, err := store.Download(context.Background(), client, "tenant-alpha", "receipt-1001", "invoice-1001.txt")
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if string(got.Body) != "receipt 1001\n" {
		t.Fatalf("Body = %q", got.Body)
	}
	if !body.closed {
		t.Fatal("object body was not closed")
	}
	if got.Key != "tenants/tenant-alpha/receipts/receipt-1001/invoice-1001.txt" {
		t.Fatalf("Key = %q", got.Key)
	}
}

func TestStoreDownloadMapsNoSuchKey(t *testing.T) {
	client := &fakeS3Client{getErr: &types.NoSuchKey{}}
	store := newTestStore(t)

	_, err := store.Download(context.Background(), client, "tenant-alpha", "receipt-404", "missing.txt")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("Download() error = %v, want ErrObjectNotFound", err)
	}
}

func TestStoreDownloadRejectsEmptyBody(t *testing.T) {
	client := &fakeS3Client{getOutput: &s3.GetObjectOutput{}}
	store := newTestStore(t)

	_, err := store.Download(context.Background(), client, "tenant-alpha", "receipt-404", "missing.txt")
	if !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("Download() error = %v, want ErrObjectNotFound", err)
	}
}

func TestStoreListTenantReceiptsUsesTenantPrefix(t *testing.T) {
	client := &fakeS3Client{listOutput: &s3.ListObjectsV2Output{Contents: []types.Object{
		{Key: aws.String("tenants/tenant-alpha/receipts/receipt-1001/invoice-1001.txt"), Size: aws.Int64(13)},
		{Key: aws.String("tenants/tenant-alpha/receipts/receipt-1002/invoice-1002.txt"), Size: aws.Int64(13)},
	}}}
	store := newTestStore(t)

	got, err := store.ListTenantReceipts(context.Background(), client, "tenant-alpha")
	if err != nil {
		t.Fatalf("ListTenantReceipts() error = %v", err)
	}
	if client.listInput == nil {
		t.Fatal("ListObjectsV2 input = nil")
	}
	if gotPrefix := aws.ToString(client.listInput.Prefix); gotPrefix != "tenants/tenant-alpha/receipts/" {
		t.Fatalf("Prefix = %q", gotPrefix)
	}
	want := []StoredObject{
		{TenantID: "tenant-alpha", ReceiptID: "receipt-1001", FileName: "invoice-1001.txt", Key: "tenants/tenant-alpha/receipts/receipt-1001/invoice-1001.txt", Size: 13},
		{TenantID: "tenant-alpha", ReceiptID: "receipt-1002", FileName: "invoice-1002.txt", Key: "tenants/tenant-alpha/receipts/receipt-1002/invoice-1002.txt", Size: 13},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListTenantReceipts() = %#v, want %#v", got, want)
	}
}

func TestStoreDeleteUsesObjectKey(t *testing.T) {
	client := &fakeS3Client{}
	store := newTestStore(t)

	if err := store.Delete(context.Background(), client, "tenant-alpha", "receipt-1001", "invoice-1001.txt"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if got := aws.ToString(client.deleteInput.Key); got != "tenants/tenant-alpha/receipts/receipt-1001/invoice-1001.txt" {
		t.Fatalf("Delete key = %q", got)
	}
}

func TestStorePresignDownloadUsesTTLAndGET(t *testing.T) {
	presigner := &fakePresigner{}
	store, err := NewStore("tenant-receipts", 5*time.Minute)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	got, err := store.PresignDownload(context.Background(), presigner, "tenant-alpha", "receipt-1001", "invoice-1001.txt")
	if err != nil {
		t.Fatalf("PresignDownload() error = %v", err)
	}
	if got.Method != http.MethodGet || got.ExpiresIn != 5*time.Minute {
		t.Fatalf("PresignDownload() = %#v", got)
	}
	if gotKey := aws.ToString(presigner.getInput.Key); gotKey != "tenants/tenant-alpha/receipts/receipt-1001/invoice-1001.txt" {
		t.Fatalf("presign key = %q", gotKey)
	}
	if presigner.expires != 5*time.Minute {
		t.Fatalf("presign expires = %s", presigner.expires)
	}
}

func TestStoreRejectsUnsafeKeys(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Upload(context.Background(), &fakeS3Client{}, ReceiptDocument{
		TenantID:  "tenant-alpha",
		ReceiptID: "receipt-1001",
		FileName:  "../secret.txt",
		Body:      []byte("secret"),
	})
	if !errors.Is(err, ErrInvalidObject) {
		t.Fatalf("Upload() error = %v, want ErrInvalidObject", err)
	}
}

func TestStorePropagatesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeS3Client{}
	store := newTestStore(t)

	_, err := store.Upload(ctx, client, SampleDocuments()[0])
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Upload() error = %v, want context.Canceled", err)
	}
	if client.putInput != nil {
		t.Fatal("PutObject should not be called after context cancellation")
	}
}

func newTestStore(t *testing.T) Store {
	t.Helper()
	store, err := NewStore("tenant-receipts", 15*time.Minute)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}

type fakeS3Client struct {
	putInput    *s3.PutObjectInput
	getInput    *s3.GetObjectInput
	listInput   *s3.ListObjectsV2Input
	deleteInput *s3.DeleteObjectInput
	putErr      error
	getErr      error
	listErr     error
	deleteErr   error
	getOutput   *s3.GetObjectOutput
	listOutput  *s3.ListObjectsV2Output
}

func (c *fakeS3Client) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	c.putInput = input
	if c.putErr != nil {
		return nil, c.putErr
	}
	return &s3.PutObjectOutput{}, nil
}

func (c *fakeS3Client) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	c.getInput = input
	if c.getErr != nil {
		return nil, c.getErr
	}
	if c.getOutput == nil {
		return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(nil))}, nil
	}
	return c.getOutput, nil
}

func (c *fakeS3Client) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	c.listInput = input
	if c.listErr != nil {
		return nil, c.listErr
	}
	if c.listOutput == nil {
		return &s3.ListObjectsV2Output{}, nil
	}
	return c.listOutput, nil
}

func (c *fakeS3Client) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	c.deleteInput = input
	if c.deleteErr != nil {
		return nil, c.deleteErr
	}
	return &s3.DeleteObjectOutput{}, nil
}

type fakePresigner struct {
	getInput *s3.GetObjectInput
	expires  time.Duration
}

func (p *fakePresigner) PresignGetObject(_ context.Context, input *s3.GetObjectInput, opts ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	p.getInput = input
	options := s3.PresignOptions{}
	for _, opt := range opts {
		opt(&options)
	}
	p.expires = options.Expires
	return &v4.PresignedHTTPRequest{Method: http.MethodGet, URL: "http://floci.local/tenant-receipts?X-Amz-Signature=test"}, nil
}

type trackingReadCloser struct {
	*bytes.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}
