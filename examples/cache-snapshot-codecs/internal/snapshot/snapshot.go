// Package snapshot은 versioned serialization과 compression 조합을 보여준다.
package snapshot

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/bluetape4k/bluetape-go/compression"
	"github.com/bluetape4k/bluetape-go/serialization"
)

// ProductSnapshot은 product read model을 위한 versioned cache snapshot이다.
type ProductSnapshot struct {
	Version   int             `json:"version"`
	Tenant    string          `json:"tenant"`
	CreatedAt time.Time       `json:"created_at"`
	Products  []ProductRecord `json:"products"`
}

// ProductRecord는 snapshot에 저장되는 cache representation이다.
type ProductRecord struct {
	ID         string `json:"id"`
	PriceCents int    `json:"price_cents"`
	Inventory  int    `json:"inventory"`
}

// EncodedSnapshot은 compressed byte와 작은 algorithm label을 담는다.
type EncodedSnapshot struct {
	Algorithm string
	Data      []byte
	Original  int
}

// Save는 product snapshot을 serialize하고 compress한다.
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

// Load는 product snapshot을 decompress하고 deserialize한다.
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

// StreamCopy는 더 큰 snapshot을 위한 stream decompression을 보여준다.
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
