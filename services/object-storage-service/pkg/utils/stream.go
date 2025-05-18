package utils

import (
	"crypto/md5"
	"encoding/hex"
	"hash"
	"io"
)

// TeeReader that computes a hash while reading
type HashingReader struct {
	reader io.Reader
	hash   io.Writer
}

// Read reads data from the underlying reader and updates the hash
func (hr *HashingReader) Read(p []byte) (n int, err error) {
	n, err = hr.reader.Read(p)
	if n > 0 {
		hr.hash.Write(p[:n])
	}
	return
}

// CreateHashingReader creates a reader that calculates a hash while reading
func CreateHashingReader(reader io.Reader, hash io.Writer) *HashingReader {
	return &HashingReader{
		reader: reader,
		hash:   hash,
	}
}

// StreamAndCalculateMD5 streams from a reader and calculates MD5 hash
// Returns the checksum and a reader that can be used to stream the data elsewhere
func StreamAndCalculateMD5(reader io.Reader) (string, io.Reader, error) {
	hash := md5.New()
	hashingReader := CreateHashingReader(reader, hash)

	// Return the hasher and the tee reader
	return "", hashingReader, nil
}

// GetMD5FromHashingReader gets the MD5 hash from a hashing reader
// Call this after streaming is complete
func GetMD5FromHashingReader(hash hash.Hash) string {
	return hex.EncodeToString(hash.Sum(nil))
}
