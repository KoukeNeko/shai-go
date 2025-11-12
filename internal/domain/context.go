package domain

// ContextSnapshot holds environment data injected into prompts and logs.
type ContextSnapshot struct {
	WorkingDir      string
	Shell           string
	OS              string
	User            string
	Files           []FileInfo
	AvailableTools  []string
	Git             *GitStatus
	Kubernetes      *KubeStatus
	EnvironmentVars map[string]string
}

// FileInfo is a minimal representation of discovered files.
type FileInfo struct {
	Path string
	Size int64
	Type FileType
}

// FileType describes the type of file entry.
type FileType string

const (
	FileTypeUnknown FileType = "unknown"
	FileTypeFile    FileType = "file"
	FileTypeDir     FileType = "dir"
	FileTypeSymlink FileType = "symlink"
)

// GitStatus captures contextual Git data.
type GitStatus struct {
	Branch             string
	ModifiedCount      int
	UntrackedCount     int
	HasUnpushedCommits bool
	Summary            string
}

// KubeStatus captures contextual Kubernetes data.
type KubeStatus struct {
	Context    string
	Namespace  string
	Namespaces []string
}
