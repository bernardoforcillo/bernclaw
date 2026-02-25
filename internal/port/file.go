package port

// FileService defines operations for file management
type FileService interface {
	ReadFile(path string) (string, error)
	WriteFile(path string, content string) error
	ListFiles(path string) ([]string, error)
	DeleteFile(path string) error
	MoveFile(source, dest string) error
	CopyFile(source, dest string) error
	CreateDirectory(path string) error
}
