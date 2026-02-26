use std::{ffi::c_char, sync::Arc};

use arrow::ffi::{FFI_ArrowArray, FFI_ArrowSchema};
use arrow_array::{Array, ArrayRef, RecordBatch, StructArray};

/// Writes `e` as a null-terminated C string into `*error_msg_out` and returns -1.
/// The caller (Go/C) is responsible for freeing the string with `csv_error_free`.
unsafe fn propagate_error(error_msg_out: *mut *mut c_char, e: impl std::fmt::Display) -> i64 {
    if !error_msg_out.is_null() {
        unsafe {
            error_msg_out
                .write(std::ffi::CString::new(e.to_string()).unwrap_or_default().into_raw());
        }
    }
    -1
}

// Opaque handles for Arrow's C Data structs.
// We use void-like structs here so cbindgen creates opaque pointers.
#[repr(C)]
pub struct ArrowSchema {
    _data: [u8; 0],
}
#[repr(C)]
pub struct ArrowArray {
    _data: [u8; 0],
}

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
    let column1: ArrayRef = Arc::new(arrow_array::Int32Array::from(vec![1, 2, 3]));
    let column2: ArrayRef = Arc::new(arrow_array::StringArray::from(vec!["a", "b", "c"]));

    let batch = RecordBatch::try_from_iter(vec![
        ("column1".to_string(), column1),
        ("column2".to_string(), column2),
    ]);

    let batch = match batch {
        Ok(b) => b,
        Err(e) => return unsafe { propagate_error(error_msg_out, e) },
    };

    let num_rows = batch.num_rows() as i64;
    let array_data = StructArray::from(batch).into_data();

    let (ffi_array_val, ffi_schema_val) = match arrow::ffi::to_ffi(&array_data) {
        Ok(pair) => pair,
        Err(e) => return unsafe { propagate_error(error_msg_out, e) },
    };

    unsafe {
        std::ptr::write(out_array as *mut FFI_ArrowArray, ffi_array_val);
        std::ptr::write(out_schema as *mut FFI_ArrowSchema, ffi_schema_val);
    }

    num_rows
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
