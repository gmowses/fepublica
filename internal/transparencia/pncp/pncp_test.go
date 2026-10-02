package pncp

import "testing"

func TestShouldFetchNextPage(t *testing.T) {
	tests := []struct {
		name        string
		page        int
		recordCount int
		envelope    pageResponse
		wantNext    bool
		wantErr     bool
	}{
		{
			name:        "last reported page",
			page:        1,
			recordCount: PageSize,
			envelope:    pageResponse{TotalPaginas: intPtr(1)},
		},
		{
			name:        "total pages takes precedence when remaining is omitted",
			page:        1,
			recordCount: PageSize,
			envelope:    pageResponse{TotalPaginas: intPtr(2)},
			wantNext:    true,
		},
		{
			name:        "total pages takes precedence over zero remaining pages",
			page:        1,
			recordCount: PageSize,
			envelope: pageResponse{
				TotalPaginas:     intPtr(2),
				PaginasRestantes: intPtr(0),
			},
			wantNext: true,
		},
		{
			name:        "bare array fallback continues after a full page",
			page:        1,
			recordCount: PageSize,
			wantNext:    true,
		},
		{
			name:        "bare array fallback stops after a short page",
			page:        1,
			recordCount: PageSize - 1,
		},
		{
			name:        "page cap returns an error when more pages remain",
			page:        maxPages,
			recordCount: PageSize,
			envelope:    pageResponse{TotalPaginas: intPtr(maxPages + 1)},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNext, err := shouldFetchNextPage(tt.page, tt.recordCount, tt.envelope)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
			if gotNext != tt.wantNext {
				t.Fatalf("fetch next = %t, want %t", gotNext, tt.wantNext)
			}
		})
	}
}

func intPtr(value int) *int {
	return &value
}
