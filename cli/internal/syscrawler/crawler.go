package syscrawler

/*
// We no longer need CFLAGS: -I... because crawler.h is in this exact folder!

// We STILL need LDFLAGS to tell it where the compiled Rust binary is.
// Notice the path: up out of syscrawler, internal, cli, then into crawler
// -Wl,-rpath embeds the runtime search path so the dynamic linker finds
// libcrawler.so when running test binaries and the built executable.
#cgo LDFLAGS: -L${SRCDIR}/../../../crawler/target/debug -lcrawler -Wl,-rpath,${SRCDIR}/../../../crawler/target/debug

#include "crawler.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"io"
	"unsafe"

	"github.com/apache/arrow/go/v18/arrow"
	"github.com/apache/arrow/go/v18/arrow/cdata"
)

// Opaque Go wrapper for the C pointer
type CrawlerHandle struct {
	ptr *C.CsvCrawlerHandle
}

// OpenCrawler is the safe Go wrapper around the FFI function
func OpenCrawler(filePath string, batchSize int, delimiter byte, filter string) (*CrawlerHandle, error) {
	// 1. Convert Go strings to C strings (must be freed!)
	cFilePath := C.CString(filePath)
	defer C.free(unsafe.Pointer(cFilePath))

	cFilter := C.CString(filter)
	defer C.free(unsafe.Pointer(cFilter))

	// 2. Setup the config struct
	config := C.CsvCrawlerConfig{
		file_path:    cFilePath,
		batch_size:   C.uintptr_t(batchSize),
		delimiter:    C.uint8_t(delimiter),
		filter_query: cFilter,
	}

	// 3. Prepare error pointer
	var cErrorMsg *C.char
	defer C.free(unsafe.Pointer(cErrorMsg))

	// 4. Call Rust over FFI
	handlePtr := C.csv_crawler_open(&config, &cErrorMsg)

	// 5. Check for errors
	if cErrorMsg != nil {
		return nil, fmt.Errorf("rust crawler failed: %s", C.GoString(cErrorMsg))
	}

	if handlePtr == nil {
		return nil, fmt.Errorf("crawler failed to open without providing an error message")
	}

	return &CrawlerHandle{ptr: handlePtr}, nil
}

func (h *CrawlerHandle) NextBatch() (arrow.Record, error) {
	var rawSchema cdata.CArrowSchema
	var rawArray cdata.CArrowArray

	schemaPtr := (*C.ArrowSchema)(unsafe.Pointer(&rawSchema))
	arrayPtr := (*C.ArrowArray)(unsafe.Pointer(&rawArray))

	var cErrMsg *C.char
	defer C.free(unsafe.Pointer(cErrMsg)) // Free error message if set 

	// Call the Rust function to get the next batch
	rowCount := C.csv_crawler_next_batch(h.ptr, schemaPtr, arrayPtr, &cErrMsg)

	if cErrMsg != nil {
		msg := C.GoString(cErrMsg)
		return nil, fmt.Errorf("rust: %s", msg)
	}

	// If rowCount is 0, we are at the end of the data
	if rowCount == 0 {
		return nil, io.EOF
	}

	rec, err := cdata.ImportCRecordBatch(&rawArray, &rawSchema)
	if err != nil {
		return nil, fmt.Errorf("arrow import failed: %w", err)
	}

	return rec, nil
}

// Free MUST be called by the user via defer
func (h *CrawlerHandle) Free() {
	if h.ptr != nil {
		C.csv_crawler_free(h.ptr)
		h.ptr = nil
	}
}
