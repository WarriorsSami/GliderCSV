use std::{ffi::c_char, io::Seek, sync::Arc};

use arrow::{
    array::RecordBatchReader,
    ffi::{FFI_ArrowArray, FFI_ArrowSchema},
};
use arrow_array::{Array, StructArray};

/// Writes `e` as a null-terminated C string into `*error_msg_out` and returns -1.
/// The caller (Go/C) is responsible for freeing the string with `csv_error_free`.
unsafe fn propagate_error(error_msg_out: *mut *mut c_char, e: impl std::fmt::Display) -> i64 {
    if !error_msg_out.is_null() {
        unsafe {
            error_msg_out.write(
                std::ffi::CString::new(e.to_string())
                    .unwrap_or_default()
                    .into_raw(),
            );
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
    reader: Box<dyn RecordBatchReader + Send>,
}

#[repr(C)]
pub struct CsvCrawlerConfig {
    pub file_path: *const c_char,
    pub batch_size: usize,
    pub delimiter: u8,
    pub filter_query: *const c_char,
}

/// # Safety
/// - `config` must be a valid pointer to a `CsvCrawlerConfig` struct with valid C strings for `file_path` and `filter_query`.
/// - `error_msg_out` must be a valid pointer to a `*mut c_char` where the function can write an error message if needed. The caller is responsible for freeing this string with `csv_error_free`.
#[no_mangle]
pub unsafe extern "C" fn csv_crawler_open(
    config: *const CsvCrawlerConfig,
    error_msg_out: *mut *mut c_char,
) -> *mut CsvCrawlerHandle {
    if config.is_null() {
        unsafe {
            propagate_error(error_msg_out, "Config pointer is null");
        }
        return std::ptr::null_mut();
    }

    let config = unsafe { &*config };

    // Convert C strings to Rust strings
    let file_path = unsafe {
        if config.file_path.is_null() {
            propagate_error(error_msg_out, "file_path is null");
            return std::ptr::null_mut();
        }
        std::ffi::CStr::from_ptr(config.file_path)
            .to_string_lossy()
            .into_owned()
    };

    // Open the CSV file
    let mut file = match std::fs::File::open(&file_path) {
        Ok(f) => f,
        Err(e) => {
            unsafe {
                propagate_error(error_msg_out, format!("Failed to open file: {} - {}", e, file_path));
            }
            return std::ptr::null_mut();
        }
    };

    // Infer schema from the CSV file
    let (schema, _records_count) = match arrow::csv::reader::Format::default()
        .with_delimiter(config.delimiter)
        .with_header(true)
        .infer_schema(&mut file, Some(config.batch_size))
    {
        Ok(s) => s,
        Err(e) => {
            unsafe {
                propagate_error(error_msg_out, format!("Failed to create CSV reader: {}", e));
            }
            return std::ptr::null_mut();
        }
    };

    // Rewind the file after schema inference
    match file.rewind() {
        Ok(_) => (),
        Err(e) => {
            unsafe {
                propagate_error(error_msg_out, format!("Failed to rewind file: {}", e));
            }
            return std::ptr::null_mut();
        }
    }

    // Create the CSV reader with the inferred schema
    let reader = match arrow::csv::reader::ReaderBuilder::new(Arc::new(schema))
        .with_delimiter(config.delimiter)
        .with_header(true)
        .with_batch_size(config.batch_size)
        .build(file)
    {
        Ok(r) => r,
        Err(e) => {
            unsafe {
                propagate_error(error_msg_out, format!("Failed to create CSV reader: {}", e));
            }
            return std::ptr::null_mut();
        }
    };

    // Create the crawler handle and return it as a raw pointer
    let handle = CsvCrawlerHandle {
        reader: Box::new(reader),
    };
    Box::into_raw(Box::new(handle))
}

/// # Safety
/// - `handle` must be a valid pointer returned by `csv_crawler_open` and not yet freed by `csv_crawler_free`.
/// - `out_schema` and `out_array` must be valid pointers to uninitialized memory where the function can write the output schema and array.
/// - `error_msg_out` must be a valid pointer to a `*mut c_char` where the function can write an error message if needed. The caller is responsible for freeing this string with `
#[no_mangle]
pub unsafe extern "C" fn csv_crawler_next_batch(
    handle: *mut CsvCrawlerHandle,
    out_schema: *mut ArrowSchema,
    out_array: *mut ArrowArray,
    error_msg_out: *mut *mut c_char,
) -> i64 {
    if handle.is_null() {
        return unsafe { propagate_error(error_msg_out, "Handle pointer is null") };
    }

    let handle = unsafe { &mut *handle };

    // Get the next batch of records
    let batch = match handle.reader.next() {
        None => return 0, // No more batches
        Some(Ok(b)) => b,
        Some(Err(e)) => return unsafe { propagate_error(error_msg_out, e) },
    };

    // Convert the RecordBatch to Arrow's C Data format
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

/// # Safety
/// - `handle` must be a valid pointer returned by `csv_crawler_open` and not yet freed by `csv_crawler_free`.
#[no_mangle]
pub unsafe extern "C" fn csv_crawler_free(handle: *mut CsvCrawlerHandle) {
    if !handle.is_null() {
        unsafe {
            // Reconstruct the Box to let Rust drop it and free memory
            let _ = Box::from_raw(handle);
        }
    }
}

#[cfg(test)]
mod tests {
    use std::{ffi::CString, mem::MaybeUninit, ptr};

    use arrow::ffi::{from_ffi, FFI_ArrowArray, FFI_ArrowSchema};
    use arrow_array::StructArray;

    use super::*;

    /// Returns the absolute path to crawler/testdata/sample.csv as a CString.
    fn sample_csv_path() -> CString {
        CString::new(format!(
            "{}/../testdata/sample.csv",
            env!("CARGO_MANIFEST_DIR")
        ))
        .unwrap()
    }

    /// Opens sample.csv and asserts no error. Panics on failure.
    fn open_sample() -> *mut CsvCrawlerHandle {
        let path = sample_csv_path();
        let filter = CString::new("").unwrap();
        let config = CsvCrawlerConfig {
            file_path: path.as_ptr(),
            batch_size: 100,
            delimiter: b',',
            filter_query: filter.as_ptr(),
        };
        let mut err: *mut c_char = ptr::null_mut();
        let handle = unsafe { csv_crawler_open(&config, &mut err) };
        assert!(err.is_null(), "csv_crawler_open returned an error");
        assert!(!handle.is_null(), "csv_crawler_open returned a null handle");
        handle
    }

    /// Reads one batch from `handle`. Returns (row_count, ffi_array, ffi_schema).
    /// The caller is responsible for dropping the returned FFI structs to fire
    /// the Arrow release callbacks and free Rust heap memory.
    unsafe fn read_one_batch(
        handle: *mut CsvCrawlerHandle,
    ) -> (i64, FFI_ArrowArray, FFI_ArrowSchema) {
        let mut out_array = MaybeUninit::<FFI_ArrowArray>::uninit();
        let mut out_schema = MaybeUninit::<FFI_ArrowSchema>::uninit();
        let mut err: *mut c_char = ptr::null_mut();

        let row_count = unsafe {
            csv_crawler_next_batch(
                handle,
                out_schema.as_mut_ptr() as *mut ArrowSchema,
                out_array.as_mut_ptr() as *mut ArrowArray,
                &mut err,
            )
        };

        assert!(err.is_null(), "csv_crawler_next_batch returned an error");
        // Safety: csv_crawler_next_batch initialises both structs on success (row_count >= 0).
        (row_count, out_array.assume_init(), out_schema.assume_init())
    }

    // ---- open tests -------------------------------------------------------

    #[test]
    fn test_open_valid_csv() {
        let handle = open_sample();
        unsafe { csv_crawler_free(handle) };
    }

    #[test]
    fn test_open_missing_file() {
        let path = CString::new("/nonexistent/path/file.csv").unwrap();
        let filter = CString::new("").unwrap();
        let config = CsvCrawlerConfig {
            file_path: path.as_ptr(),
            batch_size: 100,
            delimiter: b',',
            filter_query: filter.as_ptr(),
        };
        let mut err: *mut c_char = ptr::null_mut();
        let handle = unsafe { csv_crawler_open(&config, &mut err) };

        assert!(handle.is_null(), "expected null handle for missing file");
        assert!(!err.is_null(), "expected an error message for missing file");

        // Free the Rust-allocated error string.
        unsafe {
            let _ = CString::from_raw(err);
        }
    }

    // ---- next_batch tests -------------------------------------------------

    #[test]
    fn test_next_batch_row_count() {
        let handle = open_sample();
        let (row_count, ffi_array, ffi_schema) = unsafe { read_one_batch(handle) };

        assert_eq!(row_count, 3, "expected 3 rows");

        // Drop FFI structs — fires Arrow release callbacks, freeing Rust heap buffers.
        drop(ffi_array);
        drop(ffi_schema);

        unsafe { csv_crawler_free(handle) };
    }

    #[test]
    fn test_next_batch_schema() {
        let handle = open_sample();
        let (row_count, ffi_array, ffi_schema) = unsafe { read_one_batch(handle) };
        assert_eq!(row_count, 3);

        // Reconstruct ArrayData from the C Data Interface pair to inspect the schema.
        let array_data = unsafe { from_ffi(ffi_array, &ffi_schema) }.expect("from_ffi failed");
        // ffi_schema still needs to be dropped (from_ffi takes array by value, schema by ref).
        drop(ffi_schema);

        let struct_array = StructArray::from(array_data);
        let schema = struct_array.fields();

        assert_eq!(schema.len(), 2, "expected 2 fields");
        assert_eq!(schema[0].name(), "column1");
        assert_eq!(schema[1].name(), "column2");

        unsafe { csv_crawler_free(handle) };
    }

    // ---- EOF test ---------------------------------------------------------

    #[test]
    fn test_eof_returns_zero() {
        let handle = open_sample();

        // Consume the only batch (all 3 rows fit in batch_size=100).
        let (_, ffi_array, ffi_schema) = unsafe { read_one_batch(handle) };
        drop(ffi_array);
        drop(ffi_schema);

        // Second call — reader is exhausted, must return 0.
        let mut out_array = MaybeUninit::<FFI_ArrowArray>::uninit();
        let mut out_schema = MaybeUninit::<FFI_ArrowSchema>::uninit();
        let mut err: *mut c_char = ptr::null_mut();

        let row_count = unsafe {
            csv_crawler_next_batch(
                handle,
                out_schema.as_mut_ptr() as *mut ArrowSchema,
                out_array.as_mut_ptr() as *mut ArrowArray,
                &mut err,
            )
        };

        assert_eq!(row_count, 0, "expected 0 (EOF) on second call");
        assert!(err.is_null(), "expected no error on EOF");
        // out_array / out_schema were NOT written — do not assume_init().

        unsafe { csv_crawler_free(handle) };
    }
}
