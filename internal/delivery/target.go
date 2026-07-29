package delivery

import "time"

type Target struct {
	StorageKey   string
	MIMEType     string
	ETag         string
	Size         int64
	LastModified time.Time
	Visibility   string
	OriginalName string
}
