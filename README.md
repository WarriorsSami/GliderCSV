# 🚀 GliderCSV: High-Performance CSV Explorer

**GliderCSV** is a hybrid systems tool designed to explore, filter, and analyze massive CSV files without melting your CPU. It leverages a **Rust** backend for blazingly fast parsing and a **Go** frontend for a responsive, interactive terminal experience.

---

## 🏗 Architecture

GliderCSV is built on three pillars of modern systems engineering:

* **Rust Engine:** Utilizes `arrow-rs` and `datafusion` to parse CSVs and execute standard SQL queries for advanced predicate and projection pushdown.
* **Zero-Copy Bridge:** Uses the **Apache Arrow C Data Interface** to share memory buffers between Rust and Go without serialization overhead.
* **Bubble Tea TUI:** A beautiful, Elm-architecture-inspired interface for navigating data, managing long-running crawls, and visualizing schemas.

## ⚡ Key Features

* **Zero-Copy FFI:** Data flows from Rust to Go via memory pointers, not JSON or Protobuf.
* **SQL-Powered Pushdown:** Filter and select rows at the source using standard SQL (e.g., `SELECT name, latency FROM data WHERE status = 'ERROR' AND latency > 500`). Powered by Apache DataFusion.
* **Memory Efficient:** Streams data in configurable batches; discard non-matching rows during the raw byte-parsing phase. Process a 100GB file using only a few megabytes of RAM.
* **Interactive TUI:** Real-time filtering, sorting, and progress tracking powered by Bubble Tea.

## 🚀 Getting Started

### Prerequisites

* [Rust](https://rustup.rs/) (latest stable)
* [Go](https://go.dev/dl/) (1.21+)
* `cbindgen` (`cargo install --force cbindgen`)

### Build & Run

1. **Compile Rust Core:**
```bash
cd crawler && cargo build --release

```


*This triggers `build.rs` to automatically generate `crawler.h` inside the Go internal package.*
2. **Launch the TUI:**
```bash
cd cli && go run main.go --file data.csv

```



## 🛠 Development Workflow

This project uses `cbindgen` to ensure type safety across the FFI boundary. The Rust backend exports a C-compatible API, which is wrapped by a low-level Go `internal/syscrawler` package.

```text
GliderCSV/
├── crawler/                # The Rust backend (DataFusion, Tokio, Arrow)
│   ├── src/lib.rs          # Exports extern "C" functions
│   ├── build.rs            # Runs cbindgen to sync Go headers
│   └── Cargo.toml
├── cli/                    # The Go frontend (Bubble Tea TUI)
    ├── internal/
    │   └── syscrawler/     # Cgo bridge & cbindgen output
    ├── ui/                 # Bubble Tea models and views
    └── main.go             # Entrypoint

```

## 🗺 Roadmap

### Phase 1: Foundations (Current)

* [x] Basic Rust-to-Go FFI scaffold via `cgo`.
* [x] Automated header generation with `cbindgen`.
* [ ] Initial Bubble Tea TUI viewport setup.

### Phase 2: The Data Plane

* [x] Implement **Apache Arrow C Data Interface** for zero-copy memory transfers.
* [x] Establish strict memory lifecycles (Rust allocates, Go reads & triggers release callbacks).
* [ ] Integrate a background worker in Go (`tea.Cmd`) to keep the TUI responsive during FFI batch fetching.

### Phase 3: The Intelligence Layer (Powered by DataFusion)

* [ ] Integrate **Apache DataFusion** as the core SQL execution engine.
* [ ] Leverage DataFusion for automatic CSV schema inference (Int, Float, String, Date).
* [ ] Implement **Predicate & Projection Pushdown**: Stream raw CSV bytes directly through SQL `WHERE` clauses, discarding non-matching rows before they hit Arrow heap memory.

### Phase 4: UX & Polishing

* [ ] Add interactive SQL query input inside the Bubble Tea TUI.
* [ ] Implement "Export to Parquet" functionality.
* [ ] Dynamic column sorting and horizontal scrolling for massive datasets.

## 🏗 Detailed System Architecture

GliderCSV is designed around a strict separation of concerns, ensuring that the heavy CPU computation of parsing CSVs never blocks the UI thread. The system is divided into four distinct layers across two memory spaces.

### 1. The User Interface Layer (Go / Bubble Tea)

This is the purely idiomatic Go side of the application, completely isolated from C pointers or Rust internals.

* **The TUI Loop:** Powered by Bubble Tea. It handles keystrokes, renders the terminal UI (tables, progress bars), and maintains the application state.
* **Background Workers (`tea.Cmd`):** To prevent UI freezing, the TUI dispatches background commands that asynchronously request the next chunk of CSV rows from the Rust backend.
* **The High-Level Client:** A pure Go interface that accepts standard strings and returns safe Go errors and native Arrow `array.Record` batches.

### 2. The FFI Bridge (CGO & C ABI)

This is the boundary layer where memory is manually managed and types are strictly translated.

* **The `syscrawler` Package:** The *only* package in the Go codebase permitted to import `C`. It allocates C-strings, invokes the Rust FFI functions, and enforces strict memory cleanup via `defer C.free`.
* **The `crawler.h` Header:** Generated automatically by `cbindgen`. It acts as the strict, compile-time contract between Go and Rust, defining the shared C-structs and exported function signatures.

### 3. The Core Engine (Rust)

This is the backend processing engine, optimized for cache locality, parallelism, and speed.

* **FFI Exports (`lib.rs`):** The `#[no_mangle] extern "C"` functions that receive C-structs from Go and unwrap them into safe Rust types.
* **DataFusion SQL Engine:** Replaces manual AST evaluation. Compiles standard SQL queries into highly optimized physical execution plans, evaluating filters at the raw byte level.
* **Tokio Async Runtime:** Manages the asynchronous execution stream of DataFusion. It safely blocks the background FFI thread to orchestrate data generation while keeping the Go main UI thread responsive.

### 4. The Data Plane (Apache Arrow)

This layer handles the actual data payload, completely bypassing serialization overhead.

* **Zero-Copy Buffers:** When the DataFusion engine finds matching rows, it writes them directly into Apache Arrow columnar memory buffers on the Rust heap.
* **The Handoff:** Rust passes the raw memory addresses of these buffers (via `ArrowSchema` and `ArrowArray` C-structs) back through the FFI Bridge.
* **Cooperative Memory Management:** Go wraps these addresses using the `arrow/cdata` library, reading the data natively. Once the UI is done rendering the batch, Go calls `.Release()`, which fires a C-function pointer back into Rust, allowing the Rust allocator to safely drop the memory.