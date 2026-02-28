package syscrawler

import (
	"errors"
	"io"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/apache/arrow/go/v18/arrow/array"
)

// sampleCSVPath returns the absolute path to the shared CSV test fixture.
//
// runtime.Caller(0) resolves to this source file's compile-time absolute path,
// making the result cwd-independent regardless of how the binary is invoked:
// go test task, bare `go test` in a terminal, or an LLDB/Delve debug session.
func sampleCSVPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// thisFile = .../GliderCSV/cli/internal/syscrawler/crawler_test.go
	// navigate up three levels to the workspace root, then into testdata/
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(root, "testdata", "sample.csv")
}

// TestOpenCrawler_ValidFile checks that csv_crawler_open succeeds on a valid
// CSV file and that csv_crawler_free cleanly deallocates the handle.
func TestOpenCrawler_ValidFile(t *testing.T) {
	h, err := OpenCrawler(sampleCSVPath(t), 100, ',', "")
	if err != nil {
		t.Fatalf("OpenCrawler: %v", err)
	}
	defer h.Free()
}

// TestNextBatch_RealCSV exercises the full pipeline:
//
//	OpenCrawler → NextBatch → validate schema + data → rec.Release()
//
// The Arrow CSV reader infers integers as Int64, not Int32.
func TestNextBatch_RealCSV(t *testing.T) {
	h, err := OpenCrawler(sampleCSVPath(t), 100, ',', "")
	if err != nil {
		t.Fatalf("OpenCrawler: %v", err)
	}
	defer h.Free()

	rec, err := h.NextBatch()
	if err != nil {
		t.Fatalf("NextBatch: %v", err)
	}
	defer rec.Release()

	// --- Shape ---

	if got := rec.NumRows(); got != 3 {
		t.Errorf("NumRows: got %d, want 3", got)
	}
	if got := rec.NumCols(); got != 2 {
		t.Errorf("NumCols: got %d, want 2", got)
	}

	// --- Schema ---

	schema := rec.Schema()
	if got := schema.Field(0).Name; got != "column1" {
		t.Errorf("Field(0).Name: got %q, want %q", got, "column1")
	}
	if got := schema.Field(1).Name; got != "column2" {
		t.Errorf("Field(1).Name: got %q, want %q", got, "column2")
	}

	// --- Data: column1 (Int64 — arrow CSV inference default for integers) ---

	col0, ok := rec.Column(0).(*array.Int64)
	if !ok {
		t.Fatalf("Column(0): expected *array.Int64, got %T", rec.Column(0))
	}
	for i, want := range []int64{1, 2, 3} {
		if got := col0.Value(i); got != want {
			t.Errorf("column1[%d]: got %d, want %d", i, got, want)
		}
	}

	// --- Data: column2 (Utf8 string) ---

	col1, ok := rec.Column(1).(*array.String)
	if !ok {
		t.Fatalf("Column(1): expected *array.String, got %T", rec.Column(1))
	}
	for i, want := range []string{"a", "b", "c"} {
		if got := col1.Value(i); got != want {
			t.Errorf("column2[%d]: got %q, want %q", i, got, want)
		}
	}
}

// TestNextBatch_EOF checks that a second NextBatch call after all rows have
// been consumed returns io.EOF with a nil record.
func TestNextBatch_EOF(t *testing.T) {
	h, err := OpenCrawler(sampleCSVPath(t), 100, ',', "")
	if err != nil {
		t.Fatalf("OpenCrawler: %v", err)
	}
	defer h.Free()

	// Consume the only batch.
	rec, err := h.NextBatch()
	if err != nil {
		t.Fatalf("first NextBatch: %v", err)
	}
	rec.Release()

	// Next call must signal EOF.
	rec2, err := h.NextBatch()
	if !errors.Is(err, io.EOF) {
		t.Errorf("second NextBatch: expected io.EOF, got err=%v rec=%v", err, rec2)
	}
	if rec2 != nil {
		rec2.Release()
		t.Error("second NextBatch: expected nil record on EOF")
	}
}
