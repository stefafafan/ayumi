package main

type config struct {
	StorageDir string
	Heading    string
}

type promptEntry struct {
	Timestamp string `json:"timestamp"`
	Prompt    string `json:"prompt"`
}

type gitContext struct {
	Root     string
	GitDir   string
	RepoID   string
	BranchID string
}
