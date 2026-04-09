package transparencia

// RawRecord is a thin wrapper around a single record returned by the API.
// Collectors store the raw JSON bytes for canonical hashing; typed structs
// are used only for metadata extraction (external_id, dates).
type RawRecord struct {
	ExternalID string
	Raw        []byte // raw JSON of the record as returned by the API
}

// FetchResult is returned by source-specific Fetch methods.
type FetchResult struct {
	Source     string
	FetchedAt  string // RFC3339 timestamp for logging/debug
	Records    []RawRecord
	TotalPages int
	TotalBytes int64
	APIVersion string
}
