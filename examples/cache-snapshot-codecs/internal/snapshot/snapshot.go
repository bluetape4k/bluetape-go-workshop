// Package snapshot demonstrates versioned serialization plus compression.
package snapshot

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/bluetape4k/bluetape-go/compression"
	"github.com/bluetape4k/bluetape-go/serialization"
)

// ProductSnapshot is a versioned cache snapshot for product read models.
type ProductSnapshot struct {
	Version   int             `json:"version"`
	Tenant    string          `json:"tenant"`
	CreatedAt time.Time       `json:"created_at"`
	Products  []ProductRecord `json:"products"`
}

// ProductRecord is the cache representation stored in a snapshot.
type ProductRecord struct {
	ID         string `json:"id"`
	PriceCents int    `json:"price_cents"`
	Inventory  int    `json:"inventory"`
}

// EncodedSnapshot carries compressed bytes and a small algorithm label.
type EncodedSnapshot struct {
	Algorithm string
	Data      []byte
	Original  int
}

// Save serializes and compresses a product snapshot.
func Save(value ProductSnapshot, compressor compression.Compressor) (EncodedSnapshot, error) {
	if compressor == nil {
		return EncodedSnapshot{}, fmt.Errorf("compressor must not be nil")
	}

	serializer, err := serializer()
	if err != nil {
		return EncodedSnapshot{}, err
	}
	payload, err := serializer.Marshal(value)
	if err != nil {
		return EncodedSnapshot{}, err
	}
	compressed, err := compressor.Compress(payload)
	if err != nil {
		return EncodedSnapshot{}, err
	}
	return EncodedSnapshot{
		Algorithm: compressor.Name(),
		Data:      compressed,
		Original:  len(payload),
	}, nil
}

// Load decompresses and deserializes a product snapshot.
func Load(encoded EncodedSnapshot, compressor compression.Compressor) (ProductSnapshot, error) {
	var zero ProductSnapshot
	if compressor == nil {
		return zero, fmt.Errorf("compressor must not be nil")
	}
	payload, err := compressor.Decompress(encoded.Data)
	if err != nil {
		return zero, err
	}

	serializer, err := serializer()
	if err != nil {
		return zero, err
	}
	return serializer.Unmarshal(payload)
}

// StreamCopy demonstrates stream decompression for larger snapshots.
func StreamCopy(encoded EncodedSnapshot, compressor compression.Compressor, writer io.Writer) error {
	if writer == nil {
		return fmt.Errorf("writer must not be nil")
	}
	reader, err := compressor.NewReader(bytes.NewReader(encoded.Data))
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()
	_, err = io.Copy(writer, reader)
	return err
}

func serializer() (serialization.VersionedSerializer[ProductSnapshot], error) {
	jsonSerializer := serialization.NewJSONSerializer[ProductSnapshot](serialization.WithDisallowUnknownFields())
	return serialization.NewVersionedSerializer[ProductSnapshot](jsonSerializer, 1)
}
