use axum::{
    body::{to_bytes, Body},
    http::{header, Request, StatusCode},
    response::Response,
    Router,
};
use base64::{engine::general_purpose::STANDARD, Engine};
use folio_image::{service, Config, Service};
use futures_util::future::join_all;
use serde_json::{json, Value};
use std::path::Path;
use tempfile::TempDir;
use tower::ServiceExt;

const TOKEN: &str = "integration-test-token-with-enough-entropy";
const KEY: [u8; 32] = [42; 32];
const PNG: &str = include_str!("sample.png.base64");

fn config(directory: &Path) -> Config {
    Config {
        directory: directory.into(),
        token: TOKEN.into(),
        signing_key: KEY.to_vec(),
        max_upload: 1024,
        max_pixels: 16_000_000,
        cache_bytes: 1_048_576,
        cpu_jobs: 2,
        uploads: 4,
        vips_threads: 1,
        ai_workers: 0,
        api_rps: 1000,
        transform_rps: 1000,
    }
}

async fn app(directory: &TempDir) -> Service {
    let _ = tracing_subscriber::fmt().with_test_writer().try_init();
    service(config(directory.path())).await.unwrap()
}

async fn send(
    router: &Router,
    method: &str,
    uri: &str,
    headers: &[(&str, &str)],
    body: impl Into<Body>,
) -> Response {
    let mut builder = Request::builder().method(method).uri(uri);
    for (key, value) in headers {
        builder = builder.header(*key, *value);
    }
    router
        .clone()
        .oneshot(builder.body(body.into()).unwrap())
        .await
        .unwrap()
}

async fn get(router: &Router, uri: &str) -> Response {
    send(router, "GET", uri, &[], Body::empty()).await
}

async fn manage(router: &Router, method: &str, uri: &str, payload: Option<Value>) -> Response {
    let auth = format!("Bearer {TOKEN}");
    send(
        router,
        method,
        uri,
        &[
            ("authorization", &auth),
            ("content-type", "application/json"),
        ],
        payload.map(|v| v.to_string()).unwrap_or_default(),
    )
    .await
}

async fn data(response: Response) -> Value {
    let bytes = to_bytes(response.into_body(), 10_000_000).await.unwrap();
    serde_json::from_slice(&bytes).unwrap()
}

async fn upload(router: &Router, arguments: &str, mime: &str, bytes: Vec<u8>) -> Response {
    send(
        router,
        "POST",
        &format!("/v1/assets/upload?{arguments}"),
        &[
            ("authorization", &format!("Bearer {TOKEN}")),
            ("content-type", mime),
        ],
        bytes,
    )
    .await
}

fn sample() -> Vec<u8> {
    STANDARD.decode(PNG.trim()).unwrap()
}

async fn new_asset(router: &Router, arguments: &str) -> Value {
    let response = upload(router, arguments, "image/png", sample()).await;
    assert_eq!(response.status(), StatusCode::CREATED);
    data(response).await
}

async fn publish(router: &Router, id: &str) {
    assert_eq!(
        manage(
            router,
            "POST",
            &format!("/v1/assets/{id}/publish"),
            Some(json!({"reference":"episode:1"}))
        )
        .await
        .status(),
        200
    );
}

#[tokio::test]
async fn asset_download_range_validator_and_durable_publication() {
    let directory = tempfile::tempdir().unwrap();
    let service = app(&directory).await;
    let router = &service.router;
    assert_eq!(get(router, "/readyz").await.status(), 200);
    assert_eq!(get(router, "/v1/assets").await.status(), 401);
    let asset = new_asset(router, "name=scene.png").await;
    assert_eq!(asset["width"], 20);
    assert_eq!(asset["height"], 12);
    let id = asset["id"].as_str().unwrap();
    let uri = format!("/i/{id}");
    assert_eq!(get(router, &uri).await.status(), 403);
    let digest_uri = format!("/i/{}", asset["sha256"].as_str().unwrap());
    assert_eq!(get(router, &digest_uri).await.status(), 404);
    let signed = data(
        manage(
            router,
            "GET",
            &format!("/v1/assets/{id}/sign?format=webp&w=6"),
            None,
        )
        .await,
    )
    .await;
    let signed_url = signed["url"].as_str().unwrap();
    assert_eq!(
        get(router, &signed_url.replace("w=6", "w=7"))
            .await
            .status(),
        403
    );
    let response = get(router, signed_url).await;
    assert_eq!(response.status(), 200);
    assert_eq!(
        response.headers()[header::CACHE_CONTROL],
        "private, no-store"
    );
    let response = manage(router, "GET", &format!("/v1/assets/{id}/download"), None).await;
    assert!(response.headers().contains_key(header::CONTENT_DISPOSITION));
    assert_eq!(
        to_bytes(response.into_body(), 1000).await.unwrap(),
        sample()
    );
    publish(router, id).await;
    let by_digest = get(router, &digest_uri).await;
    assert_eq!(by_digest.status(), 200);
    assert_eq!(
        to_bytes(by_digest.into_body(), 1000).await.unwrap(),
        sample()
    );
    assert_eq!(
        get(router, &format!("{digest_uri}?sig=bad")).await.status(),
        404
    );
    let response = get(router, &uri).await;
    assert_eq!(
        response.headers()[header::CACHE_CONTROL],
        "public, max-age=31536000, immutable"
    );
    assert!(response.headers().contains_key(header::LAST_MODIFIED));
    let tag = response.headers()[header::ETAG].to_str().unwrap();
    let cached = send(
        router,
        "GET",
        &uri,
        &[("if-none-match", &format!("\"different\", W/{tag}"))],
        Body::empty(),
    )
    .await;
    assert_eq!(cached.status(), 304);
    let range = send(
        router,
        "GET",
        &uri,
        &[("range", "bytes=1-5"), ("if-range", tag)],
        Body::empty(),
    )
    .await;
    assert_eq!(range.status(), 206);
    assert_eq!(
        range.headers()[header::CONTENT_RANGE],
        format!("bytes 1-5/{}", sample().len())
    );
    assert_eq!(
        to_bytes(range.into_body(), 1000).await.unwrap(),
        sample()[1..6]
    );
    assert_eq!(
        send(
            router,
            "GET",
            &uri,
            &[("range", "bytes=99999-")],
            Body::empty()
        )
        .await
        .status(),
        416
    );
    assert_eq!(
        manage(router, "DELETE", &format!("/v1/assets/{id}"), None)
            .await
            .status(),
        409
    );
    drop(service);
    let restarted = app(&directory).await;
    assert_eq!(get(&restarted.router, &uri).await.status(), 200);
}

#[tokio::test]
async fn directories_moves_search_versions_and_delete() {
    let directory = tempfile::tempdir().unwrap();
    let service = app(&directory).await;
    let router = &service.router;
    let dir = data(
        manage(
            router,
            "POST",
            "/v1/directories",
            Some(json!({"name":"Extracts"})),
        )
        .await,
    )
    .await;
    let dir_id = dir["id"].as_str().unwrap();
    assert_eq!(
        data(manage(router, "GET", "/v1/directories?parent_id=root", None).await).await["items"][0]
            ["id"],
        dir_id
    );
    let one = new_asset(router, &format!("name=first.png&directory_id={dir_id}")).await;
    let id = one["id"].as_str().unwrap();
    let version = new_asset(router, &format!("name=second.png&previous_id={id}")).await;
    assert_eq!(version["previous_id"], id);
    assert_eq!(version["storage_key"], one["storage_key"]);
    assert_eq!(
        manage(router, "DELETE", &format!("/v1/directories/{dir_id}"), None)
            .await
            .status(),
        409
    );
    let updated = manage(
        router,
        "PATCH",
        &format!("/v1/assets/{id}"),
        Some(json!({"name":"renamed.png","metadata":{"label":"Rain"}})),
    )
    .await;
    assert_eq!(updated.status(), 200);
    let listed = data(manage(router, "GET", "/v1/assets?q=rain&limit=1", None).await).await;
    assert_eq!(listed["items"][0]["name"], "renamed.png");
    let page = data(manage(router, "GET", "/v1/assets?limit=1", None).await).await;
    let next = page["next"].as_i64().unwrap();
    assert_eq!(
        data(
            manage(
                router,
                "GET",
                &format!("/v1/assets?limit=1&after={next}"),
                None
            )
            .await
        )
        .await["items"][0]["id"],
        version["id"]
    );
    assert_eq!(
        manage(
            router,
            "POST",
            &format!("/v1/assets/{id}/move"),
            Some(json!({"directory_id":null}))
        )
        .await
        .status(),
        200
    );
    assert_eq!(
        manage(router, "DELETE", &format!("/v1/directories/{dir_id}"), None)
            .await
            .status(),
        204
    );
    assert_eq!(
        manage(router, "DELETE", &format!("/v1/assets/{id}"), None)
            .await
            .status(),
        204
    );
    assert_eq!(
        manage(router, "GET", &format!("/v1/assets/{id}"), None)
            .await
            .status(),
        404
    );
    assert_eq!(
        manage(
            router,
            "GET",
            &format!("/v1/assets/{}/download", version["id"].as_str().unwrap()),
            None
        )
        .await
        .status(),
        200
    );
}

#[tokio::test]
async fn magic_size_and_paths_are_validated() {
    let directory = tempfile::tempdir().unwrap();
    let service = app(&directory).await;
    let router = &service.router;
    assert_eq!(
        upload(router, "name=trap.png", "image/png", b"<svg/>".to_vec())
            .await
            .status(),
        415
    );
    assert_eq!(
        upload(router, "name=trap.png", "image/jpeg", sample())
            .await
            .status(),
        415
    );
    assert_eq!(
        upload(router, "name=huge.png", "image/png", vec![0; 1100])
            .await
            .status(),
        413
    );
    assert_eq!(
        upload(router, "name=..%2Ftrap.png", "image/png", sample())
            .await
            .status(),
        400
    );
    assert_eq!(
        upload(
            router,
            "name=partial.png",
            "image/png",
            sample()[..30].to_vec()
        )
        .await
        .status(),
        400
    );
    assert_eq!(get(router, "/i/%2e%2e%2fetc%2fpasswd").await.status(), 404);
}

#[tokio::test]
async fn concurrent_upload_and_transform_are_deduplicated() {
    let directory = tempfile::tempdir().unwrap();
    let mut configuration = config(directory.path());
    configuration.cpu_jobs = 4;
    let service = service(configuration).await.unwrap();
    let router = &service.router;
    let assets = join_all((0..4).map(|i| async move {
        let name = format!("name={i}.png");
        new_asset(router, &name).await
    }))
    .await;
    assert!(assets
        .iter()
        .all(|a| a["storage_key"] == assets[0]["sha256"]));
    let id = assets[0]["id"].as_str().unwrap();
    publish(router, id).await;
    let uri = format!("/i/{id}?w=9&h=4&fit=smart&format=webp&blur=1&sharpen=1");
    let variants = join_all((0..12).map(|_| get(router, &uri))).await;
    for response in variants {
        assert_eq!(response.status(), 200);
        assert_eq!(response.headers()[header::CONTENT_TYPE], "image/webp");
        let bytes = to_bytes(response.into_body(), 1000).await.unwrap();
        assert_eq!(&bytes[8..12], b"WEBP");
    }
    let stats = data(manage(router, "GET", "/v1/stats", None).await).await;
    assert_eq!(stats["cache_misses"], 1);
    assert_eq!(stats["cache_hits"], 11);
    assert_eq!(
        std::fs::read_dir(directory.path().join("derivatives"))
            .unwrap()
            .count(),
        1
    );
    let codecs = [
        ("png", "image/png"),
        ("jpeg", "image/jpeg"),
        ("avif", "image/avif"),
    ];
    for (format, mime) in codecs {
        let uri = format!("/i/{id}?crop=1,1,10,8&w=6&h=4&fit=fill&format={format}");
        let response = get(router, &uri).await;
        assert_eq!(response.status(), 200, "{format} encoder");
        assert_eq!(response.headers()[header::CONTENT_TYPE], mime);
        let bytes = to_bytes(response.into_body(), 5000).await.unwrap();
        let result = data(
            upload(
                router,
                &format!("name=transformed.{format}"),
                mime,
                bytes.to_vec(),
            )
            .await,
        )
        .await;
        assert_eq!(result["width"], 6, "{format} decode {result}");
        assert_eq!(result["height"], 4);
    }
    let watermark = get(router, &format!("/i/{id}?preset=thumb&wm={id}")).await;
    assert_eq!(watermark.status(), 200);
}

#[tokio::test]
async fn orientation_is_applied_and_decode_limit_checked() {
    let directory = tempfile::tempdir().unwrap();
    let service = app(&directory).await;
    let router = &service.router;
    let jpeg = STANDARD
        .decode(include_str!("oriented.jpg.base64").trim())
        .unwrap();
    let response = upload(router, "name=orientation.jpg", "image/jpeg", jpeg).await;
    assert_eq!(response.status(), 201);
    let original = data(response).await;
    assert_eq!(
        (
            original["width"].as_i64(),
            original["height"].as_i64(),
            original["orientation"].as_i64()
        ),
        (Some(12), Some(20), Some(6))
    );
    let id = original["id"].as_str().unwrap();
    publish(router, id).await;
    assert_eq!(
        manage(
            router,
            "POST",
            &format!("/v1/assets/{id}/prewarm?format=png"),
            None
        )
        .await
        .status(),
        204
    );
    let response = get(router, &format!("/i/{id}?format=png")).await;
    let bytes = to_bytes(response.into_body(), 4096).await.unwrap();
    assert_eq!(u32::from_be_bytes(bytes[16..20].try_into().unwrap()), 12);
    assert_eq!(u32::from_be_bytes(bytes[20..24].try_into().unwrap()), 20);

    let constrained = tempfile::tempdir().unwrap();
    let mut configuration = config(constrained.path());
    configuration.max_pixels = 200;
    let limited = folio_image::service(configuration).await.unwrap();
    assert_eq!(
        upload(&limited.router, "name=over.png", "image/png", sample())
            .await
            .status(),
        400
    );
}
