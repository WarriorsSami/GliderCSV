# 🚀 GliderCSV: High-Performance CSV Explorer

**GliderCSV** is a hybrid systems tool designed to explore, filter, and analyze massive CSV files without melting your CPU. It leverages a **Rust** backend for blazingly fast parsing and a **Go** frontend for a responsive, interactive terminal experience.

---

## 🏗 Architecture

GliderCSV is built on three pillars of modern systems engineering:

* **Rust Engine:** Utilizes `arrow-rs` and `pest` to parse CSVs and execute a custom DSL for predicate pushdown filtering.
* **Zero-Copy Bridge:** Uses the **Apache Arrow C Data Interface** to share memory buffers between Rust and Go without serialization overhead.
* **Bubble Tea TUI:** A beautiful, Elm-architecture-inspired interface for navigating data, managing long-running crawls, and visualizing schemas.

## ⚡ Key Features

* **Zero-Copy FFI:** Data flows from Rust to Go via memory pointers, not JSON or Protobuf.
* **Predicate Pushdown:** Filter rows at the source using a minimal DSL (e.g., `status == "ERROR" AND latency > 500`).
* **Memory Efficient:** Streams data in configurable batches; process a 100GB file using only 50MB of RAM.
* **Interactive TUI:** Real-time filtering, sorting, and progress tracking powered by Bubble Tea.

## 🚀 Getting Started

### Prerequisites

* [Rust](https://rustup.rs/) (latest stable)
* [Go](https://go.dev/dl/) (1.21+)
* `cbindgen` (`cargo install --force cbindgen`)

### Build & Run

1.  **Compile Rust Core:**
    ```bash
    cd crawler && cargo build --release
    ```
    *This triggers `build.rs` to automatically generate `crawler.h` inside the Go internal package.*

2.  **Launch the TUI:**
    ```bash
    cd cli && go run main.go --file data.csv
    ```

## 🛠 Development Workflow

This project uses `cbindgen` to ensure type safety across the FFI boundary. The Rust backend exports a C-compatible API, which is wrapped by a low-level Go `internal/syscrawler` package.

```text
GliderCSV/
├── crawler/                # The Rust backend (CSV logic & DSL)
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
- [x] Basic Rust-to-Go FFI scaffold via `cgo`.
- [x] Automated header generation with `cbindgen`.
- [ ] Initial Bubble Tea TUI viewport setup.

### Phase 2: The Data Plane
- [ ] Implement **Apache Arrow C Data Interface** for zero-copy transfers.
- [ ] Add streaming chunk support (`batch_size`) to prevent OOM on large files.
- [ ] Integrate a background worker in Go to keep the TUI responsive during crawls.

### Phase 3: The Intelligence Layer
- [ ] Implement the **Pest**-based DSL for predicate pushdown.
- [ ] Add CSV schema auto-detection (inferring types: Int, Float, String, Date).
- [ ] Support for multiple delimiters and Gzip/Zstd compressed CSVs.

### Phase 4: UX & Polishing
- [ ] Add interactive column sorting in the TUI.
- [ ] Implement "Export to Parquet" functionality.
- [ ] Live-reload mode for monitoring growing log files.