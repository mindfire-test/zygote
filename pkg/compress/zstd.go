// Package compress provides high-performance Zstandard (zstd) compression utilities.
package compress

import (
	"bytes"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// ZstdMagic is the 4-byte magic header identifying a standard Zstandard frame (0xFD2FB528 little-endian).
var ZstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}

// IsCompressed checks if the data begins with the Zstandard magic header.
func IsCompressed(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], ZstdMagic)
}

var (
	encoderPool = sync.Pool{
		New: func() any {
			enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
			if err != nil {
				panic(err)
			}
			return enc
		},
	}
	decoderPool = sync.Pool{
		New: func() any {
			dec, err := zstd.NewReader(nil)
			if err != nil {
				panic(err)
			}
			return dec
		},
	}
)

// Compress compresses src using Zstandard.
func Compress(src []byte) []byte {
	enc := encoderPool.Get().(*zstd.Encoder)
	defer encoderPool.Put(enc)
	return enc.EncodeAll(src, make([]byte, 0, len(src)))
}

// Decompress decompresses a Zstandard-compressed payload.
func Decompress(src []byte) ([]byte, error) {
	dec := decoderPool.Get().(*zstd.Decoder)
	defer decoderPool.Put(dec)
	return dec.DecodeAll(src, nil)
}
