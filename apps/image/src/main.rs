// Only the imported public manifest is routable. Private previews live outside it.
use axum::{
    extract::{Path, Query, State},
    http::{header, HeaderMap, StatusCode},
    response::{IntoResponse, Response},
    routing::get,
    Router,
};
use hmac::{Hmac, Mac};
use sha2::Sha256;
use std::{
    collections::{HashMap, HashSet},
    sync::Arc,
    time::{SystemTime, UNIX_EPOCH},
};

struct Library {
    paths: HashSet<String>,
    key: Vec<u8>,
}
fn error(status: StatusCode) -> Response {
    (status, [(header::CACHE_CONTROL, "private, no-store")]).into_response()
}

async fn read(
    State(lib): State<Arc<Library>>,
    Path(path): Path<String>,
    Query(args): Query<HashMap<String, String>>,
    headers: HeaderMap,
) -> Response {
    let private = path.starts_with("image-preview/");
    let asset = path.strip_prefix("image-preview/").unwrap_or(&path);
    if !lib.paths.contains(asset) {
        return error(StatusCode::NOT_FOUND);
    }
    if private {
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        let exp = args
            .get("exp")
            .and_then(|s| s.parse::<u64>().ok())
            .unwrap_or(0);
        let sig = args
            .get("sig")
            .and_then(|s| hex::decode(s).ok())
            .unwrap_or_default();
        let mut mac = Hmac::<Sha256>::new_from_slice(&lib.key).unwrap();
        mac.update(format!("/{path}\n{exp}").as_bytes());
        if exp <= now || exp > now + 600 || mac.verify_slice(&sig).is_err() {
            return error(StatusCode::FORBIDDEN);
        }
    }
    let relative = if private {
        format!("preview/{asset}.webp")
    } else {
        asset.to_owned()
    };
    let tag = format!(
        "\"{}{}\"",
        asset.rsplit('/').next().unwrap(),
        if private { "-preview-v1" } else { "" }
    );
    let mut res = if !private
        && headers
            .get(header::IF_NONE_MATCH)
            .and_then(|h| h.to_str().ok())
            == Some(&tag)
    {
        StatusCode::NOT_MODIFIED.into_response()
    } else {
        match tokio::fs::read(format!("library/{relative}")).await {
            Ok(bytes) => bytes.into_response(),
            Err(_) => return error(StatusCode::SERVICE_UNAVAILABLE),
        }
    };
    let h = res.headers_mut();
    h.insert(
        header::CONTENT_TYPE,
        if private || asset.ends_with(".webp") {
            "image/webp"
        } else {
            "image/jpeg"
        }
        .parse()
        .unwrap(),
    );
    h.insert(header::ETAG, tag.parse().unwrap());
    h.insert(
        header::CACHE_CONTROL,
        if private {
            "private, no-store"
        } else {
            "public, max-age=31536000, immutable"
        }
        .parse()
        .unwrap(),
    );
    h.insert("x-folio-image", "gz-naruto-v1".parse().unwrap());
    res
}
#[tokio::main]
async fn main() -> std::io::Result<()> {
    let paths = std::fs::read_to_string("manifest.txt")?
        .lines()
        .map(str::to_owned)
        .collect();
    let key = std::fs::read("preview.key")?;
    assert!(key.len() >= 32);
    let app = Router::new()
        .route("/{*path}", get(read))
        .with_state(Arc::new(Library { paths, key }));
    axum::serve(tokio::net::TcpListener::bind("127.0.0.1:14580").await?, app).await
}
