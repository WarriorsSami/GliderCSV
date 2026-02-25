use std::env;
use std::path::PathBuf;

fn main() {
    let crate_dir = env::var("CARGO_MANIFEST_DIR").unwrap();
    println!("cargo:rerun-if-changed=src/lib.rs");

    // 1. Start at the rust-crawler directory
    let mut out_path = PathBuf::from(&crate_dir);
    
    // 2. Navigate up to the root, then down into the Go syscrawler package
    out_path.push("../cli/internal/syscrawler/crawler.h");

    // 3. Generate and write directly to the Go folder
    cbindgen::Builder::new()
        .with_crate(crate_dir)
        .with_language(cbindgen::Language::C)
        .with_include_guard("CSV_CRAWLER_H")
        .generate()
        .expect("Unable to generate bindings")
        .write_to_file(out_path); 
}