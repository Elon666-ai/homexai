package service

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// FileUploadValidator provides secure file upload validation
type FileUploadValidator struct {
	// Allowed file extensions
	allowedExtensions map[string]bool
	// Allowed MIME types
	allowedMimeTypes map[string]bool
	// Max file size in bytes
	maxFileSize int64
	// Whether to check magic bytes
	checkMagicBytes bool
}

// NewFileUploadValidator creates a new validator with default settings
func NewFileUploadValidator() *FileUploadValidator {
	return &FileUploadValidator{
		allowedExtensions: map[string]bool{
			// Images
			".jpg":  true,
			".jpeg": true,
			".png":  true,
			".gif":  true,
			".webp": true,
			".bmp":  true,
			// Documents
			".pdf":  true,
			".doc":  true,
			".docx": true,
			".xls":  true,
			".xlsx": true,
			".ppt":  true,
			".pptx": true,
			".txt":  true,
			".csv":  true,
			// Archives
			".zip": true,
			".rar": true,
		},
		allowedMimeTypes: map[string]bool{
			// Images
			"image/jpeg":     true,
			"image/png":      true,
			"image/gif":      true,
			"image/webp":     true,
			"image/bmp":      true,
			"image/x-ms-bmp": true,
			// Documents
			"application/pdf":    true,
			"application/msword": true,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
			"application/vnd.ms-excel": true,
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
			"application/vnd.ms-powerpoint":                                             true,
			"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
			"text/plain": true,
			"text/csv":   true,
			// Archives
			"application/zip":              true,
			"application/x-rar-compressed": true,
		},
		maxFileSize:     10 * 1024 * 1024, // 10MB default
		checkMagicBytes: true,
	}
}

// SetMaxFileSize sets the maximum allowed file size
func (v *FileUploadValidator) SetMaxFileSize(size int64) {
	v.maxFileSize = size
}

// AddAllowedExtension adds an allowed file extension
func (v *FileUploadValidator) AddAllowedExtension(ext string) {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	v.allowedExtensions[strings.ToLower(ext)] = true
}

// AddAllowedMimeType adds an allowed MIME type
func (v *FileUploadValidator) AddAllowedMimeType(mimeType string) {
	v.allowedMimeTypes[strings.ToLower(mimeType)] = true
}

// ValidateFile performs comprehensive file validation
func (v *FileUploadValidator) ValidateFile(fileHeader *multipart.FileHeader) error {
	// 1. Check file size
	if err := v.validateFileSize(fileHeader); err != nil {
		return err
	}

	// 2. Sanitize and validate filename
	if err := v.validateFileName(fileHeader.Filename); err != nil {
		return err
	}

	// 3. Check file extension
	if err := v.validateExtension(fileHeader.Filename); err != nil {
		return err
	}

	// 4. Open file for content validation
	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer file.Close()

	// 5. Validate MIME type
	if err := v.validateMimeType(file); err != nil {
		return err
	}

	// 6. Check magic bytes if enabled
	if v.checkMagicBytes {
		if err := v.validateMagicBytes(file, fileHeader.Filename); err != nil {
			return err
		}
	}

	return nil
}

// validateFileSize checks if file size is within limits
func (v *FileUploadValidator) validateFileSize(fileHeader *multipart.FileHeader) error {
	if fileHeader.Size > v.maxFileSize {
		return fmt.Errorf("file size %d bytes exceeds maximum allowed size %d bytes",
			fileHeader.Size, v.maxFileSize)
	}

	if fileHeader.Size == 0 {
		return fmt.Errorf("file is empty")
	}

	return nil
}

// validateFileName sanitizes and validates the filename
func (v *FileUploadValidator) validateFileName(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is empty")
	}

	// Check for path traversal attempts
	if strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename: path traversal detected")
	}

	// Check for null bytes
	if strings.Contains(filename, "\x00") {
		return fmt.Errorf("invalid filename: null byte detected")
	}

	// Check for special characters that could be dangerous
	dangerousChars := regexp.MustCompile(`[<>:"|?*\\\/]`)
	if dangerousChars.MatchString(filename) {
		return fmt.Errorf("invalid filename: contains dangerous characters")
	}

	// Check filename length (max 255 characters)
	if len(filename) > 255 {
		return fmt.Errorf("filename too long (max 255 characters)")
	}

	return nil
}

// validateExtension checks if file extension is allowed
func (v *FileUploadValidator) validateExtension(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))

	if ext == "" {
		return fmt.Errorf("file has no extension")
	}

	if !v.allowedExtensions[ext] {
		return fmt.Errorf("file extension %s is not allowed", ext)
	}

	return nil
}

// validateMimeType checks if MIME type is allowed
func (v *FileUploadValidator) validateMimeType(file multipart.File) error {
	// Read first 512 bytes to detect MIME type
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	// Reset file pointer
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(buffer[:n])

	// Remove charset parameter if present (e.g., "text/plain; charset=utf-8" -> "text/plain")
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	if !v.allowedMimeTypes[mimeType] {
		return fmt.Errorf("MIME type %s is not allowed", mimeType)
	}

	return nil
}

// validateMagicBytes checks file magic bytes to verify file type
func (v *FileUploadValidator) validateMagicBytes(file multipart.File, filename string) error {
	// Read first 16 bytes for magic byte checking
	buffer := make([]byte, 16)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file magic bytes: %w", err)
	}

	// Reset file pointer
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	magicBytes := buffer[:n]

	// Check magic bytes based on extension
	switch ext {
	case ".jpg", ".jpeg":
		if !bytes.HasPrefix(magicBytes, []byte{0xFF, 0xD8, 0xFF}) {
			return fmt.Errorf("file content does not match JPEG format")
		}
	case ".png":
		if !bytes.HasPrefix(magicBytes, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
			return fmt.Errorf("file content does not match PNG format")
		}
	case ".gif":
		if !bytes.HasPrefix(magicBytes, []byte("GIF87a")) && !bytes.HasPrefix(magicBytes, []byte("GIF89a")) {
			return fmt.Errorf("file content does not match GIF format")
		}
	case ".pdf":
		if !bytes.HasPrefix(magicBytes, []byte("%PDF-")) {
			return fmt.Errorf("file content does not match PDF format")
		}
	case ".zip":
		if !bytes.HasPrefix(magicBytes, []byte{0x50, 0x4B, 0x03, 0x04}) &&
			!bytes.HasPrefix(magicBytes, []byte{0x50, 0x4B, 0x05, 0x06}) &&
			!bytes.HasPrefix(magicBytes, []byte{0x50, 0x4B, 0x07, 0x08}) {
			return fmt.Errorf("file content does not match ZIP format")
		}
	case ".docx", ".xlsx", ".pptx":
		// These are actually ZIP files
		if !bytes.HasPrefix(magicBytes, []byte{0x50, 0x4B, 0x03, 0x04}) &&
			!bytes.HasPrefix(magicBytes, []byte{0x50, 0x4B, 0x05, 0x06}) &&
			!bytes.HasPrefix(magicBytes, []byte{0x50, 0x4B, 0x07, 0x08}) {
			return fmt.Errorf("file content does not match Office format")
		}
	case ".rar":
		if !bytes.HasPrefix(magicBytes, []byte("Rar!\x1A\x07")) &&
			!bytes.HasPrefix(magicBytes, []byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x01, 0x00}) {
			return fmt.Errorf("file content does not match RAR format")
		}
	}

	return nil
}

// SanitizeFilename removes dangerous characters and returns a safe filename
func SanitizeFilename(filename string) string {
	// Remove path components
	filename = filepath.Base(filename)

	// Replace spaces with underscores
	filename = strings.ReplaceAll(filename, " ", "_")

	// Remove all characters except alphanumeric, underscore, hyphen, and dot
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-\.]`)
	filename = reg.ReplaceAllString(filename, "")

	// Ensure it doesn't start with a dot (hidden file)
	filename = strings.TrimPrefix(filename, ".")

	// Limit length
	if len(filename) > 200 {
		ext := filepath.Ext(filename)
		nameWithoutExt := strings.TrimSuffix(filename, ext)
		filename = nameWithoutExt[:200-len(ext)] + ext
	}

	return filename
}

// GenerateSafeFilename generates a unique, safe filename
func GenerateSafeFilename(originalFilename, uploadDir string) string {

	// Sanitize original filename
	safe := SanitizeFilename(originalFilename)
	ext := filepath.Ext(safe)
	nameWithoutExt := strings.TrimSuffix(safe, ext)

	// Generate random suffix
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	randomStr := hex.EncodeToString(randomBytes)

	// Use timestamp + random for uniqueness
	timestamp := time.Now().Unix()

	// Construct new filename
	newFilename := fmt.Sprintf("%s_%d_%s%s", nameWithoutExt, timestamp, randomStr, ext)

	return newFilename
}

// ValidateImageDimensions checks if image dimensions are within acceptable range
func ValidateImageDimensions(file multipart.File, maxWidth, maxHeight int) error {

	// Decode image
	img, _, err := image.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Reset file pointer
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if maxWidth > 0 && width > maxWidth {
		return fmt.Errorf("image width %d exceeds maximum allowed width %d", width, maxWidth)
	}

	if maxHeight > 0 && height > maxHeight {
		return fmt.Errorf("image height %d exceeds maximum allowed height %d", height, maxHeight)
	}

	return nil
}

// GetFileTypeCategory returns the category of a file based on its extension
func GetFileTypeCategory(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return "image"
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "word"
	case ".xls", ".xlsx":
		return "excel"
	case ".ppt", ".pptx":
		return "powerpoint"
	case ".txt", ".csv":
		return "text"
	case ".zip", ".rar":
		return "archive"
	default:
		return "unknown"
	}
}
