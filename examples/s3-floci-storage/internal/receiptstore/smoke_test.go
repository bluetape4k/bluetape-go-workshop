package receiptstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	flocitestcontainer "github.com/bluetape4k/bluetape-go/testcontainers/floci"
)

func TestFlociSmoke(t *testing.T) {
	if os.Getenv("BLUETAPE_S3_FLOCI_STORAGE_SMOKE") != "1" {
		t.Skip("set BLUETAPE_S3_FLOCI_STORAGE_SMOKE=1 to run the Docker-backed Floci S3 storage smoke test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	details := flocitestcontainer.Start(ctx, t, flocitestcontainer.WithS3Config(flocitestcontainer.DefaultS3Config()))
	cfg := flocitestcontainer.LoadConfig(ctx, t, details)
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = true
	})
	presigner := s3.NewPresignClient(client)

	bucket := "tenant-receipts-" + fmt.Sprintf("%d", time.Now().UnixNano())
	if _, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = client.DeleteBucket(cleanupCtx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	})

	store, err := NewStore(bucket, 15*time.Minute)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	doc := SampleDocuments()[0]
	stored, err := store.Upload(ctx, client, doc)
	if err != nil {
		t.Fatalf("Upload() smoke error = %v", err)
	}
	if stored.Key != "tenants/tenant-alpha/receipts/receipt-1001/invoice-1001.txt" {
		t.Fatalf("stored key = %q", stored.Key)
	}

	downloaded, err := store.Download(ctx, client, doc.TenantID, doc.ReceiptID, doc.FileName)
	if err != nil {
		t.Fatalf("Download() smoke error = %v", err)
	}
	if !bytes.Equal(downloaded.Body, doc.Body) {
		t.Fatalf("downloaded body = %q, want %q", downloaded.Body, doc.Body)
	}

	items, err := store.ListTenantReceipts(ctx, client, doc.TenantID)
	if err != nil {
		t.Fatalf("ListTenantReceipts() smoke error = %v", err)
	}
	if len(items) != 1 || items[0].Key != stored.Key {
		t.Fatalf("listed items = %#v, want one stored key", items)
	}

	presigned, err := store.PresignDownload(ctx, presigner, doc.TenantID, doc.ReceiptID, doc.FileName)
	if err != nil {
		t.Fatalf("PresignDownload() smoke error = %v", err)
	}
	if presigned.Method != http.MethodGet {
		t.Fatalf("presign method = %q, want GET", presigned.Method)
	}
	if parsed, err := url.Parse(presigned.URL); err != nil || parsed.Scheme == "" || parsed.Host == "" {
		t.Fatalf("presigned URL = %q parseErr=%v", presigned.URL, err)
	}

	if err := store.Delete(ctx, client, doc.TenantID, doc.ReceiptID, doc.FileName); err != nil {
		t.Fatalf("Delete() smoke error = %v", err)
	}
	if _, err := store.Download(ctx, client, doc.TenantID, doc.ReceiptID, doc.FileName); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("Download() after delete error = %v, want ErrObjectNotFound", err)
	}
}
