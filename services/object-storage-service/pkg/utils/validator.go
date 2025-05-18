package utils

import (
	"errors"
	"fmt"
	"mime/multipart"
	"net/mail"
	"path/filepath"
	"regexp"
	"strings"
)

// Validators provides validation methods
type Validators struct{}

// NewValidators creates a new validators instance
func NewValidators() *Validators {
	return &Validators{}
}

// IsValidEmail checks if a string is a valid email address
func (v *Validators) IsValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// IsValidUsername checks if a string is a valid username
func (v *Validators) IsValidUsername(username string) bool {
	// Username should be alphanumeric with underscores and dashes, 3-32 characters
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{3,32}$`, username)
	return matched
}

// IsValidFilename checks if a string is a valid filename
func (v *Validators) IsValidFilename(filename string) bool {
	// Check if the filename has invalid characters
	invalid := []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalid {
		if strings.Contains(filename, char) {
			return false
		}
	}

	// Check if the filename is too long
	if len(filename) > 255 {
		return false
	}

	return true
}

// IsValidFolderPath checks if a string is a valid folder path
func (v *Validators) IsValidFolderPath(path string) bool {
	// Path should not contain ..
	if strings.Contains(path, "..") {
		return false
	}

	// Path should not start with /
	if strings.HasPrefix(path, "/") {
		return false
	}

	// Check path segments
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if !v.IsValidFilename(part) {
			return false
		}
	}

	return true
}

// IsValidID checks if a string is a valid MongoDB ID
func (v *Validators) IsValidID(id string) bool {
	matched, _ := regexp.MatchString(`^[0-9a-fA-F]{24}$`, id)
	return matched
}

// IsAllowedFileType checks if a file has an allowed extension
func (v *Validators) IsAllowedFileType(filename string, allowedExtensions []string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if len(ext) == 0 {
		return false
	}

	// Remove the dot from the extension
	ext = ext[1:]

	for _, allowed := range allowedExtensions {
		if ext == allowed {
			return true
		}
	}

	return false
}

// IsValidImageFile checks if a file is a valid image based on extension
func (v *Validators) IsValidImageFile(filename string) bool {
	allowedExtensions := []string{"jpg", "jpeg", "png", "gif", "webp", "svg"}
	return v.IsAllowedFileType(filename, allowedExtensions)
}

// IsValidDocumentFile checks if a file is a valid document based on extension
func (v *Validators) IsValidDocumentFile(filename string) bool {
	allowedExtensions := []string{"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt"}
	return v.IsAllowedFileType(filename, allowedExtensions)
}

// IsValidAudioFile checks if a file is a valid audio based on extension
func (v *Validators) IsValidAudioFile(filename string) bool {
	allowedExtensions := []string{"mp3", "wav", "ogg", "m4a", "flac"}
	return v.IsAllowedFileType(filename, allowedExtensions)
}

// IsValidVideoFile checks if a file is a valid video based on extension
func (v *Validators) IsValidVideoFile(filename string) bool {
	allowedExtensions := []string{"mp4", "avi", "mov", "wmv", "mkv", "webm"}
	return v.IsAllowedFileType(filename, allowedExtensions)
}

// ValidateFileSize checks if a file size is within limits
func (v *Validators) ValidateFileSize(fileSize int64, maxSize int64) error {
	if fileSize > maxSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", maxSize)
	}
	return nil
}

// ValidateFileHeader performs basic validation on file header
func (v *Validators) ValidateFileHeader(fileHeader *multipart.FileHeader, maxSize int64) error {
	if fileHeader == nil {
		return errors.New("no file provided")
	}

	if fileHeader.Size == 0 {
		return errors.New("file is empty")
	}

	if fileHeader.Size > maxSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", maxSize)
	}

	if !v.IsValidFilename(fileHeader.Filename) {
		return errors.New("invalid filename")
	}

	return nil
}

// ValidateAvatarFile validates an avatar file
func (v *Validators) ValidateAvatarFile(fileHeader *multipart.FileHeader) error {
	// Check file header
	if err := v.ValidateFileHeader(fileHeader, 5*1024*1024); err != nil { // 5MB max
		return err
	}

	// Check file type
	if !v.IsValidImageFile(fileHeader.Filename) {
		return errors.New("avatar must be an image file (jpg, jpeg, png, gif, webp, svg)")
	}

	return nil
}

// IsSafeString checks if a string contains only safe characters
func (v *Validators) IsSafeString(s string) bool {
	// Check for potential SQL injection or command injection
	dangerous := []string{"'", "\"", ";", "--", "/*", "*/", "xp_", "="}
	for _, char := range dangerous {
		if strings.Contains(s, char) {
			return false
		}
	}

	return true
}
