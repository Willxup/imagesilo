package imagealias

import "time"

type Alias struct {
	ID        string
	Path      string
	ImageID   string
	Source    string
	CreatedAt time.Time
}

type Page struct {
	Items      []Alias
	NextCursor string
}
