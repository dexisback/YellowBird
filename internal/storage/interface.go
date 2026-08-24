package storage

//NOTE: why io.reader? because the current (the before version ) of UploadInput is tied to a multipart.File, which works for HTTP file but fails for a worker generated thumbnail.
//a worker has a normal file/reader, not a multipart request. we therefore update all with io.reader now instead of multipart because now worker is going to be using it aswell

//what this does -> defines the contract that every storage provider (cloudinary/s3/the stealth wala im gonna be using/local disk) must implement
// //scalable

import (
	"context"
	"io"
	// "io"
	// "mime/multipart"
)

type UploadInput struct {
	// File     multipart.File
	Reader   io.Reader
	FileName string
	MimeType string
	Size     int64

	// Header   *multipart.FileHeader
	// FileName string
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
	GetURL(ctx context.Context, storageKey string) (string, error)                           //add new for thumbnail.go
	Download(ctx context.Context, storageKey string, mimeType string) (io.ReadCloser, error) //retrieve file for FFmpeg, to get the actual bytes of the file

}

//download has mimetype because cloudinary sdk needs to know whether the thing is a image() or a video() (cloudinary can transport the thing but it needs to know the mimetpye first)

//download() will need to be implemented onto cloudinary.go aswell
