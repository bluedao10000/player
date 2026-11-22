use crate::{
    delivery::{error, Representation, IMMUTABLE, PRIVATE},
    storage::Reader,
};
use axum::{
    extract::{Path, Query, State},
    http::{HeaderMap, Method, StatusCode},
    response::Response,
    routing::get,
    Router,
};
use std::{
    collections::{HashMap, HashSet},
    io,
    path::PathBuf,
    sync::Arc,
    time::SystemTime,
};
use tokio::{fs, io::AsyncSeekExt};

pub struct Library {
    paths: HashSet<String>,
    key: Vec<u8>,
    directory: PathBuf,
}

impl Library {
    pub async fn open(directory: PathBuf) -> anyhow::Result<Self> {
        let manifest = fs::read_to_string(directory.join("manifest.txt")).await?;
        let key = fs::read(directory.join("preview.key")).await?;
        anyhow::ensure!(
            key.len() >= 32,
            "legacy preview key must have at least 32 bytes"
        );
        let paths = manifest.lines().map(str::to_owned).collect::<HashSet<_>>();
        anyhow::ensure!(
            paths.iter().all(|path| path.starts_with("art/")
                && path.split('/').all(|p| !p.is_empty()
                    && p != "."
                    && p != ".."
                    && p.bytes()
                        .all(|b| b.is_ascii_alphanumeric() || b"-_.".contains(&b)))),
            "invalid path in legacy manifest"
        );
        Ok(Self {
            paths,
            key,
            directory,
        })
    }

    pub fn router(self) -> Router {
        Router::new()
            .route("/{*path}", get(read))
            .with_state(Arc::new(self))
    }
}

async fn read(
    State(lib): State<Arc<Library>>,
    Path(path): Path<String>,
    Query(args): Query<HashMap<String, String>>,
    method: Method,
    headers: HeaderMap,
) -> Response {
    let private = path.starts_with("image-preview/");
    let asset = path.strip_prefix("image-preview/").unwrap_or(&path);
    if !lib.paths.contains(asset) {
        return error(StatusCode::NOT_FOUND);
    }
    if private {
        let exp = args
            .get("exp")
            .and_then(|s| s.parse::<u64>().ok())
            .unwrap_or(0);
        let sig = args.get("sig").map(String::as_str).unwrap_or_default();
        if !crate::verify(&lib.key, &format!("/{path}\n{exp}"), exp, sig) {
            return error(StatusCode::FORBIDDEN);
        }
    }
    let relative = if private {
        format!("preview/{asset}.webp")
    } else {
        asset.to_owned()
    };
    let mut file = match fs::File::open(lib.directory.join("library").join(relative)).await {
        Ok(file) => file,
        Err(_) => return error(StatusCode::SERVICE_UNAVAILABLE),
    };
    let metadata = match file.metadata().await {
        Ok(metadata) if metadata.is_file() => metadata,
        _ => return error(StatusCode::SERVICE_UNAVAILABLE),
    };
    let tag = format!(
        "\"{}{}\"",
        asset.rsplit('/').next().unwrap(),
        if private { "-preview-v1" } else { "" }
    );
    let mut response = Representation {
        tag: &tag,
        mime: if private || asset.ends_with(".webp") {
            "image/webp"
        } else {
            "image/jpeg"
        },
        cache: if private { PRIVATE } else { IMMUTABLE },
        length: metadata.len(),
        modified: metadata.modified().unwrap_or(SystemTime::UNIX_EPOCH),
    }
    .serve(
        &headers,
        method == Method::HEAD,
        !private,
        |offset| async move {
            if offset > 0 {
                file.seek(io::SeekFrom::Start(offset)).await?;
            }
            Ok(Box::new(file) as Reader)
        },
    )
    .await;
    response
        .headers_mut()
        .insert("x-folio-image", "gz-naruto-v1".parse().unwrap());
    response
}
