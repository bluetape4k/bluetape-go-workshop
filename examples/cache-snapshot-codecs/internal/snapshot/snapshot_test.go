package snapshot_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go-workshop/examples/cache-snapshot-codecs/internal/snapshot"
	"github.com/bluetape4k/bluetape-go/compression"
)

func TestSnapshotRoundTripWithCompatibilityAndInternalCodecs(t *testing.T) {
	value := sampleSnapshot()
	for _, compressor := range []compression.Compressor{compression.Gzip(), compression.Zstd()} {
		encoded, err := snapshot.Save(value, compressor)
		if err != nil {
			t.Fatalf("%s save: %v", compressor.Name(), err)
		}

		decoded, err := snapshot.Load(encoded, compressor)
		if err != nil {
			t.Fatalf("%s load: %v", compressor.Name(), err)
		}
		if decoded.Tenant != value.Tenant || len(decoded.Products) != len(value.Products) {
			t.Fatalf("%s decoded = %+v", compressor.Name(), decoded)
		}
		if encoded.Original <= len(encoded.Data) {
			t.Logf("%s produced %d bytes from %d serialized bytes; choose by workload measurement", encoded.Algorithm, len(encoded.Data), encoded.Original)
		}
	}
}

func TestSnapshotRejectsCorruptCompressedData(t *testing.T) {
	_, err := snapshot.Load(snapshot.EncodedSnapshot{Data: []byte("not gzip")}, compression.Gzip())
	if err == nil {
		t.Fatal("expected corrupt compressed data to fail")
	}
}

func TestSnapshotRejectsCorruptSerializedPayload(t *testing.T) {
	encoded, err := compression.Gzip().Compress([]byte("bad serialized envelope"))
	if err != nil {
		t.Fatalf("compress corrupt payload: %v", err)
	}
	_, err = snapshot.Load(snapshot.EncodedSnapshot{Data: encoded}, compression.Gzip())
	if err == nil {
		t.Fatal("expected corrupt serialized payload to fail")
	}
}

func TestSnapshotStreamDecompression(t *testing.T) {
	value := sampleSnapshot()
	encoded, err := snapshot.Save(value, compression.Zstd())
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	var decoded bytes.Buffer
	if err := snapshot.StreamCopy(encoded, compression.Zstd(), &decoded); err != nil {
		t.Fatalf("stream copy: %v", err)
	}
	if !strings.Contains(decoded.String(), "sku-001") {
		t.Fatalf("stream payload = %s", decoded.String())
	}
}

func sampleSnapshot() snapshot.ProductSnapshot {
	return snapshot.ProductSnapshot{
		Version:   1,
		Tenant:    "commerce",
		CreatedAt: time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		Products: []snapshot.ProductRecord{
			{ID: "sku-001", PriceCents: 1299, Inventory: 42},
			{ID: "sku-002", PriceCents: 2599, Inventory: 7},
			{ID: "sku-003", PriceCents: 3999, Inventory: 0},
		},
	}
}
