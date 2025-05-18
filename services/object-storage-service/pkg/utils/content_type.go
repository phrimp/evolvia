package utils

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
)

// ContentTypeDetector provides methods to detect content types
type ContentTypeDetector struct{}

// NewContentTypeDetector creates a new content type detector
func NewContentTypeDetector() *ContentTypeDetector {
	return &ContentTypeDetector{}
}

// DetectContentTypeFromExtension tries to detect content type from a file extension
func (d *ContentTypeDetector) DetectContentTypeFromExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return "application/octet-stream"
	}

	// Get content type from extension
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}

	return contentType
}

// DetectContentTypeFromBytes tries to detect content type from the file content
func (d *ContentTypeDetector) DetectContentTypeFromBytes(data []byte) string {
	// Use http.DetectContentType for binary detection
	return http.DetectContentType(data)
}

// DetectContentType tries to detect content type from both extension and content
func (d *ContentTypeDetector) DetectContentType(filename string, data []byte) string {
	// First try by extension
	contentType := d.DetectContentTypeFromExtension(filename)

	// If we couldn't determine the type or got the default type, try by content
	if contentType == "application/octet-stream" {
		return d.DetectContentTypeFromBytes(data)
	}

	return contentType
}

// DetectContentTypeFromReader tries to detect content type from a reader
// Note: This will read the first 512 bytes and the reader will no longer be at the beginning
func (d *ContentTypeDetector) DetectContentTypeFromReader(reader io.Reader) (string, error) {
	// Read the first 512 bytes to detect content type
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Detect content type
	contentType := http.DetectContentType(buffer[:n])

	return contentType, nil
}

// DetectContentTypeFromReaderPreserving tries to detect content type from a reader
// without changing the reader's position
func (d *ContentTypeDetector) DetectContentTypeFromReaderPreserving(reader io.ReadSeeker) (string, error) {
	// Remember current position
	currentPos, err := reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}

	// Seek to beginning
	_, err = reader.Seek(0, io.SeekStart)
	if err != nil {
		return "", err
	}

	// Read the first 512 bytes to detect content type
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Detect content type
	contentType := http.DetectContentType(buffer[:n])

	// Seek back to original position
	_, err = reader.Seek(currentPos, io.SeekStart)
	if err != nil {
		return "", err
	}

	return contentType, nil
}

// IsImageContentType checks if a content type is an image
func (d *ContentTypeDetector) IsImageContentType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/tiff", "image/bmp", "image/svg+xml":
		return true
	default:
		return false
	}
}

// IsVideoContentType checks if a content type is a video
func (d *ContentTypeDetector) IsVideoContentType(contentType string) bool {
	switch contentType {
	case "video/mp4", "video/mpeg", "video/webm", "video/ogg", "video/quicktime", "video/x-msvideo":
		return true
	default:
		return false
	}
}

// IsAudioContentType checks if a content type is an audio
func (d *ContentTypeDetector) IsAudioContentType(contentType string) bool {
	switch contentType {
	case "audio/mpeg", "audio/ogg", "audio/wav", "audio/webm", "audio/midi", "audio/x-midi":
		return true
	default:
		return false
	}
}

// IsTextContentType checks if a content type is text
func (d *ContentTypeDetector) IsTextContentType(contentType string) bool {
	switch contentType {
	case "text/plain", "text/html", "text/css", "text/javascript", "text/csv", "text/xml", "application/json", "application/xml":
		return true
	default:
		return false
	}
}

// IsDocumentContentType checks if a content type is a document
func (d *ContentTypeDetector) IsDocumentContentType(contentType string) bool {
	switch contentType {
	case "application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document", // docx
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", // xlsx
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation": // pptx
		return true
	default:
		return false
	}
}
