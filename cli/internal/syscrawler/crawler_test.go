package syscrawler

import (
	"testing"

	"github.com/apache/arrow/go/v18/arrow/array"
)

// TestNextBatch_MockData exercises the full Arrow C Data Interface handshake.
//
// The Rust csv_crawler_next_batch currently returns a hardcoded mock batch
// regardless of the handle value, so we pass a nil handle to keep the test
// independent of csv_crawler_open being unimplemented.
//
// The call chain under test:
//
//	CrawlerHandle.NextBatch()
//	  → csv_crawler_next_batch (Rust FFI, writes FFI_ArrowArray + FFI_ArrowSchema)
//	  → cdata.ImportCRecordBatch (Go takes ownership, installs release finalizer)
//	  → rec.Release() (fires Rust release callback, deallocates Arrow buffers)
func TestNextBatch_MockData(t *testing.T) {
	h := &CrawlerHandle{ptr: nil}

	rec, err := h.NextBatch()
	if err != nil {
		t.Fatalf("NextBatch returned unexpected error: %v", err)
	}
	defer rec.Release()

	// --- Shape assertions ---

	if got := rec.NumRows(); got != 3 {
		t.Errorf("NumRows: got %d, want 3", got)
	}

	if got := rec.NumCols(); got != 2 {
		t.Errorf("NumCols: got %d, want 2", got)
	}

	// --- Schema assertions ---

	schema := rec.Schema()

	if got := schema.Field(0).Name; got != "column1" {
		t.Errorf("Field(0).Name: got %q, want %q", got, "column1")
	}

	if got := schema.Field(1).Name; got != "column2" {
		t.Errorf("Field(1).Name: got %q, want %q", got, "column2")
	}

	// --- Data assertions ---

	col0, ok := rec.Column(0).(*array.Int32)
	if !ok {
		t.Fatalf("Column(0): expected *array.Int32, got %T", rec.Column(0))
	}

	wantInts := []int32{1, 2, 3}
	for i, want := range wantInts {
		if got := col0.Value(i); got != want {
			t.Errorf("column1[%d]: got %d, want %d", i, got, want)
		}
	}

	col1, ok := rec.Column(1).(*array.String)
	if !ok {
		t.Fatalf("Column(1): expected *array.String, got %T", rec.Column(1))
	}

	wantStrings := []string{"a", "b", "c"}
	for i, want := range wantStrings {
		if got := col1.Value(i); got != want {
			t.Errorf("column2[%d]: got %q, want %q", i, got, want)
		}
	}
}
