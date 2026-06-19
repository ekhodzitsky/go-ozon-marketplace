use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR")?);
    let api_dir = manifest_dir
        .join("..")
        .join("..")
        .join("api")
        .canonicalize()?;

    // Пересобираем, если меняются proto-контракты.
    println!("cargo:rerun-if-changed={}", api_dir.join("proto").display());

    let out_dir = PathBuf::from(env::var("OUT_DIR")?);
    let proto_root = out_dir.join("proto");
    fs::create_dir_all(&proto_root)?;

    // buf экспортирует proto-файлы проекта вместе с зависимостями
    // (например, buf/validate/validate.proto) во временную директорию.
    let status = Command::new("buf")
        .current_dir(&api_dir)
        .args(["export", ".", "--output", proto_root.to_str().unwrap()])
        .status()?;

    if !status.success() {
        return Err("buf export failed".into());
    }

    let protos = collect_proto_files(&proto_root);
    if protos.is_empty() {
        return Err("no proto files found".into());
    }

    tonic_build::configure()
        .build_server(false)
        .include_file("proto.rs")
        .out_dir(&out_dir)
        .compile_protos(&protos, &[proto_root])?;

    Ok(())
}

fn collect_proto_files(dir: &Path) -> Vec<PathBuf> {
    let mut files = Vec::new();
    if dir.is_dir() {
        for entry in fs::read_dir(dir).unwrap() {
            let entry = entry.unwrap();
            let path = entry.path();
            if path.is_dir() {
                files.extend(collect_proto_files(&path));
            } else if path.extension().map_or(false, |e| e == "proto") {
                files.push(path);
            }
        }
    }
    files
}
