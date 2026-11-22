mod ai;
mod assets;
mod delivery;
mod images;
mod legacy;
mod processing;
pub mod storage;

use axum::{
    extract::{DefaultBodyLimit, Request, State as ExtractState},
    http::{header, HeaderValue, StatusCode},
    middleware::{self, Next},
    response::{IntoResponse, Response},
    routing::{delete, get, post},
    Json, Router,
};
use hmac::{Hmac, Mac};
use sha2::{Digest, Sha256};
use sqlx::sqlite::{
    SqliteConnectOptions, SqliteJournalMode, SqlitePool, SqlitePoolOptions, SqliteSynchronous,
};
use std::{
    env, io,
    path::PathBuf,
    sync::{Arc, OnceLock},
    time::{Duration, SystemTime, UNIX_EPOCH},
};
use subtle::ConstantTimeEq;
use tokio::{
    fs::{self, OpenOptions},
    sync::{Mutex, Notify, Semaphore},
    time::Instant,
};

type Shared = Arc<State>;

struct State {
    db: SqlitePool,
    scratch: PathBuf,
    storage: Arc<dyn storage::ObjectStorage>,
    cache: images::Cache,
    signing_key: Vec<u8>,
    token_digest: [u8; 32],
    encoder_version: String,
    max_upload: u64,
    max_pixels: u64,
    cpu: Arc<Semaphore>,
    uploads: Semaphore,
    transform_queue: Semaphore,
    api_rate: Rate,
    transform_rate: Rate,
    jobs: Notify,
    ai: ai::Providers,
    _lock: std::fs::File,
}

pub struct Config {
    pub directory: PathBuf,
    pub token: String,
    pub signing_key: Vec<u8>,
    pub max_upload: u64,
    pub max_pixels: u64,
    pub cache_bytes: u64,
    pub cpu_jobs: usize,
    pub uploads: usize,
    pub vips_threads: i32,
    pub ai_workers: usize,
    pub api_rps: u32,
    pub transform_rps: u32,
}

impl Config {
    pub async fn from_env(directory: PathBuf) -> anyhow::Result<Self> {
        let token = match env::var_os("IMAGE_API_TOKEN_FILE") {
            Some(path) => fs::read_to_string(path).await?.trim().to_owned(),
            None => env::var("IMAGE_API_TOKEN").unwrap_or_default(),
        };
        let key = match env::var_os("IMAGE_SIGNING_KEY_FILE") {
            Some(path) => fs::read(path).await?,
            None => hex::decode(env::var("IMAGE_SIGNING_KEY_HEX").unwrap_or_default())?,
        };
        Ok(Self {
            directory,
            token,
            signing_key: key,
            max_upload: setting(
                "IMAGE_MAX_UPLOAD_BYTES",
                25 * 1024 * 1024,
                1024,
                100 * 1024 * 1024,
            )?,
            max_pixels: setting("IMAGE_MAX_PIXELS", 32_000_000, 1, 64_000_000)?,
            cache_bytes: setting("IMAGE_CACHE_BYTES", 2 * 1024 * 1024 * 1024, 0, u64::MAX)?,
            cpu_jobs: setting("IMAGE_TRANSFORM_CONCURRENCY", 2, 1, 8)? as usize,
            uploads: setting("IMAGE_UPLOAD_CONCURRENCY", 4, 1, 32)? as usize,
            vips_threads: setting("IMAGE_VIPS_THREADS", 1, 1, 8)? as i32,
            ai_workers: setting("IMAGE_AI_WORKERS", 1, 0, 4)? as usize,
            api_rps: setting("IMAGE_API_RPS", 30, 1, 5000)? as u32,
            transform_rps: setting("IMAGE_TRANSFORM_RPS", 20, 1, 5000)? as u32,
        })
    }
}

fn setting(name: &str, default: u64, min: u64, max: u64) -> anyhow::Result<u64> {
    let value = match env::var(name) {
        Ok(value) => value.parse()?,
        Err(env::VarError::NotPresent) => default,
        Err(e) => return Err(e.into()),
    };
    anyhow::ensure!(
        (min..=max).contains(&value),
        "{name} must be in {min}..={max}"
    );
    Ok(value)
}

async fn open(config: Config, ai: ai::Providers) -> anyhow::Result<Shared> {
    anyhow::ensure!(
        config.token.len() >= 32 && config.token.bytes().all(|c| c.is_ascii_graphic()),
        "configure IMAGE_API_TOKEN with at least 32 printable non-space ASCII characters"
    );
    anyhow::ensure!(
        config.signing_key.len() >= 32,
        "configure an image signing key of at least 32 bytes"
    );

    static ENCODER: OnceLock<Result<String, String>> = OnceLock::new();
    let encoder_version = ENCODER
        .get_or_init(|| processing::init(config.vips_threads).map_err(|e| e.to_string()))
        .as_ref()
        .map_err(|error| anyhow::anyhow!("{error}"))?
        .clone();
    anyhow::ensure!(
        !config.directory.to_string_lossy().contains(['[', ']']),
        "image data path cannot contain square brackets"
    );

    fs::create_dir_all(&config.directory).await?;
    let lock = OpenOptions::new()
        .read(true)
        .write(true)
        .create(true)
        .truncate(false)
        .open(config.directory.join("instance.lock"))
        .await?
        .into_std()
        .await;
    lock.try_lock()
        .map_err(|error| anyhow::anyhow!("image data directory in use: {error}"))?;
    let scratch = config.directory.join("scratch");
    if fs::try_exists(&scratch).await? {
        fs::remove_dir_all(&scratch).await?;
    }
    fs::create_dir(&scratch).await?;
    let storage = Arc::new(storage::LocalStorage::open(&config.directory, &scratch).await?);
    let cache =
        images::Cache::open(config.directory.join("derivatives"), config.cache_bytes).await?;
    let db = SqlitePoolOptions::new()
        .max_connections(6)
        .acquire_timeout(Duration::from_secs(5))
        .connect_with(
            SqliteConnectOptions::new()
                .filename(config.directory.join("library.sqlite"))
                .create_if_missing(true)
                .foreign_keys(true)
                .journal_mode(SqliteJournalMode::Wal)
                .synchronous(SqliteSynchronous::Full)
                .busy_timeout(Duration::from_secs(3)),
        )
        .await?;
    let mut migration = db.begin_with("BEGIN IMMEDIATE").await?;
    let version: i32 = sqlx::query_scalar("PRAGMA user_version")
        .fetch_one(&mut *migration)
        .await?;
    match version {
        0 => {
            sqlx::raw_sql(include_str!("schema.sql"))
                .execute(&mut *migration)
                .await?;
            sqlx::query("PRAGMA user_version=1")
                .execute(&mut *migration)
                .await?;
        }
        1 => {}
        other => anyhow::bail!("unsupported image schema version {other}"),
    }
    // A provider may have accepted a request before the process exited.
    sqlx::query("UPDATE jobs SET state='failed',error='worker_interrupted',updated_at=? WHERE state='running'")
        .bind(now() as i64).execute(&mut *migration).await?;
    migration.commit().await?;

    Ok(Arc::new(State {
        db,
        scratch,
        storage,
        cache,
        ai,
        encoder_version,
        signing_key: config.signing_key,
        token_digest: Sha256::digest(config.token).into(),
        max_upload: config.max_upload,
        max_pixels: config.max_pixels,
        cpu: Arc::new(Semaphore::new(config.cpu_jobs)),
        uploads: Semaphore::new(config.uploads),
        transform_queue: Semaphore::new(32),
        api_rate: Rate::new(config.api_rps),
        transform_rate: Rate::new(config.transform_rps),
        jobs: Notify::new(),
        _lock: lock,
    }))
}

pub struct Service {
    pub router: Router,
    workers: Vec<tokio::task::JoinHandle<()>>,
}

impl Drop for Service {
    fn drop(&mut self) {
        for worker in &self.workers {
            worker.abort();
        }
    }
}

pub async fn service(
    config: Option<Config>,
    legacy_dir: Option<PathBuf>,
) -> anyhow::Result<Service> {
    let legacy = match legacy_dir {
        Some(directory) => legacy::Library::open(directory).await?.router(),
        None => Router::new(),
    };
    let mut workers = Vec::new();
    let app = if let Some(config) = config {
        let count = config.ai_workers;
        let state = open(config, ai::Providers::load()?).await?;
        for _ in 0..count {
            workers.push(tokio::spawn(ai::worker(state.clone())));
        }
        modern(state)
    } else {
        Router::new()
            .route("/healthz", get(health))
            .route("/readyz", get(health))
    };
    Ok(Service {
        router: legacy.merge(app),
        workers,
    })
}

fn modern(state: Shared) -> Router {
    let management = Router::new()
        .route("/v1/assets/upload", post(assets::upload))
        .route("/v1/assets", get(assets::list))
        .route(
            "/v1/assets/{id}",
            get(assets::detail)
                .patch(assets::edit)
                .delete(assets::delete),
        )
        .route("/v1/assets/{id}/download", get(assets::download))
        .route("/v1/assets/{id}/move", post(assets::move_asset))
        .route("/v1/assets/{id}/publish", post(assets::publish))
        .route("/v1/assets/{id}/sign", get(images::sign))
        .route("/v1/assets/{id}/prewarm", post(images::prewarm))
        .route(
            "/v1/directories",
            get(assets::list_directories).post(assets::create_directory),
        )
        .route("/v1/directories/{id}", delete(assets::delete_directory))
        .route("/v1/ai/analyze", post(ai::analyze))
        .route("/v1/ai/generate", post(ai::generate))
        .route("/v1/ai/edit", post(ai::edit))
        .route("/v1/jobs/{id}", get(ai::status))
        .route("/v1/stats", get(stats))
        .layer(DefaultBodyLimit::max(32 * 1024))
        .route_layer(middleware::from_fn_with_state(state.clone(), authorize))
        .route_layer(middleware::from_fn(private_headers));
    management
        .route("/i/{id}", get(images::read))
        .route("/openapi.yaml", get(openapi))
        .route("/healthz", get(health))
        .route("/readyz", get(ready))
        .with_state(state)
}

async fn health() -> StatusCode {
    StatusCode::OK
}

async fn ready(ExtractState(state): ExtractState<Shared>) -> Result<StatusCode, ApiError> {
    sqlx::query("SELECT 1 FROM assets LIMIT 1")
        .execute(&state.db)
        .await
        .map_err(|_| ApiError(StatusCode::SERVICE_UNAVAILABLE, "database unavailable"))?;
    fs::metadata(&state.scratch)
        .await
        .map_err(|_| ApiError(StatusCode::SERVICE_UNAVAILABLE, "storage unavailable"))?;
    Ok(StatusCode::OK)
}

async fn openapi() -> impl IntoResponse {
    (
        [(header::CONTENT_TYPE, "application/yaml")],
        include_str!("../openapi.yaml"),
    )
}

async fn stats(ExtractState(state): ExtractState<Shared>) -> Json<serde_json::Value> {
    use std::sync::atomic::Ordering::Relaxed;
    Json(
        serde_json::json!({"cache_hits":state.cache.hits.load(Relaxed), "cache_misses":state.cache.misses.load(Relaxed)}),
    )
}

async fn authorize(
    ExtractState(state): ExtractState<Shared>,
    request: Request,
    next: Next,
) -> Response {
    // Rate-limit both authenticated operations and token guessing on the internal API.
    if !state.api_rate.allow().await {
        return ApiError::BUSY.into_response();
    }
    let token = request
        .headers()
        .get(header::AUTHORIZATION)
        .and_then(|h| h.to_str().ok())
        .and_then(|h| h.split_once(' '))
        .filter(|(scheme, _)| scheme.eq_ignore_ascii_case("bearer"))
        .map(|(_, token)| token);
    if !token.is_some_and(|token| {
        let digest: [u8; 32] = Sha256::digest(token).into();
        bool::from(state.token_digest.ct_eq(&digest))
    }) {
        let mut response = ApiError(StatusCode::UNAUTHORIZED, "invalid API token").into_response();
        response
            .headers_mut()
            .insert(header::WWW_AUTHENTICATE, HeaderValue::from_static("Bearer"));
        return response;
    }
    next.run(request).await
}

async fn private_headers(request: Request, next: Next) -> Response {
    let mut response = next.run(request).await;
    response.headers_mut().insert(
        header::CACHE_CONTROL,
        HeaderValue::from_static(delivery::PRIVATE),
    );
    response
}

struct Rate {
    state: Mutex<(Instant, f64)>,
    per_second: f64,
}

impl Rate {
    fn new(per_second: u32) -> Self {
        let rate = f64::from(per_second);
        Self {
            state: Mutex::new((Instant::now(), rate * 2.0)),
            per_second: rate,
        }
    }

    async fn allow(&self) -> bool {
        let mut state = self.state.lock().await;
        let instant = Instant::now();
        let tokens = (state.1 + instant.duration_since(state.0).as_secs_f64() * self.per_second)
            .min(self.per_second * 2.0);
        let allowed = tokens >= 1.0;
        *state = (instant, tokens - f64::from(u8::from(allowed)));
        allowed
    }
}

#[derive(Debug)]
struct ApiError(StatusCode, &'static str);

impl ApiError {
    const BUSY: Self = Self(
        StatusCode::TOO_MANY_REQUESTS,
        "capacity unavailable, retry later",
    );
    const NOT_FOUND: Self = Self(StatusCode::NOT_FOUND, "resource not found");
    fn bad(message: &'static str) -> Self {
        Self(StatusCode::BAD_REQUEST, message)
    }
    fn conflict(message: &'static str) -> Self {
        Self(StatusCode::CONFLICT, message)
    }
    fn internal(error: impl std::fmt::Display) -> Self {
        tracing::error!(error = %error, "image service error");
        Self(
            StatusCode::INTERNAL_SERVER_ERROR,
            "internal image service error",
        )
    }
    fn constraint(error: sqlx::Error) -> Self {
        if error
            .as_database_error()
            .is_some_and(|e| e.is_foreign_key_violation() || e.is_unique_violation())
        {
            Self::conflict("directory, version or name conflict")
        } else {
            error.into()
        }
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let mut response = (
            self.0,
            [(header::CACHE_CONTROL, delivery::PRIVATE)],
            Json(serde_json::json!({"error":self.1})),
        )
            .into_response();
        if self.0 == StatusCode::TOO_MANY_REQUESTS {
            response
                .headers_mut()
                .insert(header::RETRY_AFTER, HeaderValue::from_static("1"));
        }
        response
    }
}

impl From<io::Error> for ApiError {
    fn from(error: io::Error) -> Self {
        Self::internal(error)
    }
}
impl From<sqlx::Error> for ApiError {
    fn from(error: sqlx::Error) -> Self {
        Self::internal(error)
    }
}

fn now() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

fn sign(key: &[u8], message: &str) -> String {
    let mut mac = Hmac::<Sha256>::new_from_slice(key).expect("valid HMAC key");
    mac.update(message.as_bytes());
    hex::encode(mac.finalize().into_bytes())
}

fn verify(key: &[u8], message: &str, expiration: u64, signature: &str) -> bool {
    let current = now();
    if expiration <= current || expiration > current + 600 || signature.len() != 64 {
        return false;
    }
    let mut digest = [0; 32];
    if hex::decode_to_slice(signature, &mut digest).is_err() {
        return false;
    }
    let mut mac = Hmac::<Sha256>::new_from_slice(key).expect("valid HMAC key");
    mac.update(message.as_bytes());
    mac.verify_slice(&digest).is_ok()
}
