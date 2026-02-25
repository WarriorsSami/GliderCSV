use std::ffi::c_char;

// Opaque handles for Arrow's C Data structs. 
// We use void-like structs here so cbindgen creates opaque pointers.
#[repr(C)]
pub struct ArrowSchema { _data: [u8; 0] }
#[repr(C)]
pub struct ArrowArray { _data: [u8; 0] }

// The opaque handle holding our crawler state
pub struct CsvCrawlerHandle {
    // Internal Rust state goes here (e.g., File, CSV Reader)
}

#[repr(C)]
pub struct CsvCrawlerConfig {
    pub file_path: *const c_char,
    pub batch_size: usize,
    pub delimiter: u8,
    pub filter_query: *const c_char,
}

#[no_mangle]
pub extern "C" fn csv_crawler_open(
    config: *const CsvCrawlerConfig,
    error_msg_out: *mut *mut c_char,
) -> *mut CsvCrawlerHandle {
    // Scaffold: Just return a null pointer or dummy instance for now
    std::ptr::null_mut()
}

#[no_mangle]
pub extern "C" fn csv_crawler_next_batch(
    handle: *mut CsvCrawlerHandle,
    out_schema: *mut ArrowSchema,
    out_array: *mut ArrowArray,
    error_msg_out: *mut *mut c_char,
) -> i64 {
    // Scaffold: Return 0 rows read
    0
}

#[no_mangle]
pub extern "C" fn csv_crawler_free(handle: *mut CsvCrawlerHandle) {
    if !handle.is_null() {
        unsafe {
            // Reconstruct the Box to let Rust drop it and free memory
            let _ = Box::from_raw(handle);
        }
    }
}