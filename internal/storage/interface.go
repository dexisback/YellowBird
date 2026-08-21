package storage

//what this does -> defines the contract that every storage provider (cloudinary/s3/the stealth wala im gonna be using/local disk) must implement
// //scalable

import (
	"context"
	"mime/multipart"
)

type UploadInput struct {
	File     multipart.File
	Header   *multipart.FileHeader
	FileName string
}

type UploadResult struct {
	StorageKey       string
	URL              string
	OriginalFileName string

	MimeType string
	Size     int64
}

type Storage interface {
	Upload(ctx context.Context, input UploadInput) (*UploadResult, error)
	Delete(ctx context.Context, storageKey string) error
	GetURL(ctx context.Context, storageKey string) (string, error)  //add new for thumbnail.go

}
