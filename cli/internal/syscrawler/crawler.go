package syscrawler

/*
// We no longer need CFLAGS: -I... because crawler.h is in this exact folder!

// We STILL need LDFLAGS to tell it where the compiled Rust binary is.
// Notice the path: up out of syscrawler, internal, cli, then into csv_crawler
#cgo LDFLAGS: -L../../../csv_crawler/target/debug -lcrawler

#include "crawler.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
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

	// 4. Call Rust over FFI
	handlePtr := C.csv_crawler_open(&config, &cErrorMsg)

	// 5. Check for errors
	if cErrorMsg != nil {
		defer C.free(unsafe.Pointer(cErrorMsg))
		return nil, fmt.Errorf("rust crawler failed: %s", C.GoString(cErrorMsg))
	}

	if handlePtr == nil {
		return nil, fmt.Errorf("crawler failed to open without providing an error message")
	}

	return &CrawlerHandle{ptr: handlePtr}, nil
}

// Free MUST be called by the user via defer
func (h *CrawlerHandle) Free() {
	if h.ptr != nil {
		C.csv_crawler_free(h.ptr)
		h.ptr = nil
	}
}
