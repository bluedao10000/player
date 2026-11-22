use crate::{
    assets::Asset,
    delivery::{Representation, IMMUTABLE, PRIVATE},
    processing::{Format, Query, Recipe, RECIPE},
    storage::{Key, Reader, Temp},
    ApiError, Shared, State,
};
use axum::{
    extract::{OriginalUri, Path, RawQuery, State as ExtractState},
    http::{HeaderMap, Method, StatusCode},
    response::Response,
    Json,
};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::{
    collections::VecDeque,
    io,
    path::{Path as FsPath, PathBuf},
    sync::atomic::{AtomicU64, Ordering},
    time::{Duration, UNIX_EPOCH},
};
use tokio::{
    fs::{self, File},
    io::AsyncSeekExt,
    sync::Mutex,
};

pub struct Cache {
    directory: PathBuf,
    index: Mutex<Index>,
    limit: u64,
    locks: [Mutex<()>; 256],
    pub hits: AtomicU64,
    pub misses: AtomicU64,
}

#[derive(Default)]
struct Index {
    entries: VecDeque<(PathBuf, u64)>,
    bytes: u64,
}

impl Cache {
    pub async fn open(directory: PathBuf, limit: u64) -> io::Result<Self> {
        fs::create_dir_all(&directory).await?;
        let mut scan = fs::read_dir(&directory).await?;
        let mut entries = Vec::new();
        while let Some(entry) = scan.next_entry().await? {
            if entry.file_type().await?.is_file()
                && entry
                    .file_name()
                    .to_str()
                    .is_some_and(|v| Key::parse(v).is_ok())
            {
                let metadata = entry.metadata().await?;
                entries.push((metadata.modified()?, entry.path(), metadata.len()));
            }
        }
        entries.sort_unstable();
        let mut index = Index::default();
        for (_, path, size) in entries {
            index.entries.push_back((path, size));
            index.bytes += size;
        }
        index.evict(limit, 10_000).await?;
        Ok(Self {
            directory,
            index: Mutex::new(index),
            limit,
            locks: std::array::from_fn(|_| Mutex::new(())),
            hits: AtomicU64::new(0),
            misses: AtomicU64::new(0),
        })
    }

    async fn publish(&self, scratch: &FsPath, destination: PathBuf, size: u64) -> io::Result<()> {
        if size > self.limit {
            return Ok(());
        }
        let mut index = self.index.lock().await;
        index.evict(self.limit - size, 9_999).await?;
        fs::rename(scratch, &destination).await?;
        index.entries.push_back((destination, size));
        index.bytes += size;
        Ok(())
    }
}

impl Index {
    async fn evict(&mut self, bytes: u64, count: usize) -> io::Result<()> {
        while self.bytes > bytes || self.entries.len() > count {
            let (path, size) = self.entries.front().expect("nonempty cache index");
            match fs::remove_file(path).await {
                Ok(()) => {}
                Err(e) if e.kind() == io::ErrorKind::NotFound => {}
                Err(e) => return Err(e),
            }
            self.bytes -= size;
            self.entries.pop_front();
        }
        Ok(())
    }
}

fn query(raw: Option<&str>) -> Result<Query, ApiError> {
    if raw.is_some_and(|s| s.len() > 2048) {
        return Err(ApiError::bad("query too long"));
    }
    serde_urlencoded::from_str(raw.unwrap_or_default())
        .map_err(|_| ApiError::bad("invalid image query"))
}

fn signing_payload(asset: &Asset, recipe: Option<&Recipe>, exp: u64) -> String {
    format!(
        "folio-image-url-v1\n/i/{}\n{}\n{RECIPE}\n{}\n{exp}",
        asset.id,
        asset.sha256,
        recipe.map(Recipe::canonical).unwrap_or_default()
    )
}

pub async fn sign(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
    RawQuery(raw): RawQuery,
) -> Result<Json<Value>, ApiError> {
    let params = query(raw.as_deref())?;
    if params.sig.is_some() || params.exp.is_some() {
        return Err(ApiError::bad("signature parameters are reserved"));
    }
    let asset = state.asset(&id).await?;
    let recipe = recipe(&state, &params, &asset)?;
    let exp = crate::now() + 600;
    let sig = crate::sign(
        &state.signing_key,
        &signing_payload(&asset, recipe.as_ref(), exp),
    );
    let mut raw = raw.unwrap_or_default();
    if !raw.is_empty() {
        raw.push('&');
    }
    Ok(Json(
        json!({"url":format!("/i/{id}?{raw}exp={exp}&sig={sig}"),"expires_at":exp}),
    ))
}

pub async fn read(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
    OriginalUri(uri): OriginalUri,
    method: Method,
    headers: HeaderMap,
) -> Result<Response, ApiError> {
    if uri.path() != format!("/i/{id}") {
        return Err(ApiError::NOT_FOUND);
    }
    let params = query(uri.query())?;
    let asset = state.asset(&id).await?;
    let recipe = recipe(&state, &params, &asset)?;
    let signed = params.sig.is_some() || params.exp.is_some();
    let cache = if asset.visibility == "public" && !signed {
        if recipe.is_some() {
            "public, max-age=86400"
        } else {
            IMMUTABLE
        }
    } else {
        let exp = params.exp.unwrap_or(0);
        let payload = signing_payload(&asset, recipe.as_ref(), exp);
        if !crate::verify(
            &state.signing_key,
            &payload,
            exp,
            params.sig.as_deref().unwrap_or_default(),
        ) {
            return Err(ApiError(
                StatusCode::FORBIDDEN,
                "invalid or expired signature",
            ));
        }
        PRIVATE
    };
    let head = method == Method::HEAD;
    match recipe {
        Some(recipe) => derived(&state, &asset, recipe, &headers, head, cache).await,
        None => original(&state, &asset, &headers, head, cache).await,
    }
}

fn recipe(state: &State, params: &Query, asset: &Asset) -> Result<Option<Recipe>, ApiError> {
    params.recipe(
        Format::from_mime(&asset.mime)?,
        asset.width as u32,
        asset.height as u32,
        state.max_pixels,
    )
}

pub async fn original(
    state: &State,
    asset: &Asset,
    headers: &HeaderMap,
    head: bool,
    cache: &str,
) -> Result<Response, ApiError> {
    let key = Key::parse(&asset.storage_key)?;
    let tag = format!("\"{}\"", asset.sha256);
    Ok(Representation {
        tag: &tag,
        mime: &asset.mime,
        cache,
        length: asset.size as u64,
        modified: UNIX_EPOCH + Duration::from_secs(asset.created_at as u64),
    }
    .serve(headers, head, true, |offset| {
        state.storage.get_stream(&key, offset)
    })
    .await)
}

pub async fn prewarm(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
    RawQuery(raw): RawQuery,
) -> Result<StatusCode, ApiError> {
    let asset = state.asset(&id).await?;
    let params = query(raw.as_deref())?;
    if params.exp.is_some() || params.sig.is_some() {
        return Err(ApiError::bad("signature parameters are reserved"));
    }
    let recipe =
        recipe(&state, &params, &asset)?.ok_or(ApiError::bad("select a transform or preset"))?;
    derivative_file(&state, &asset, &recipe).await?;
    Ok(StatusCode::NO_CONTENT)
}

async fn derived(
    state: &State,
    asset: &Asset,
    recipe: Recipe,
    headers: &HeaderMap,
    head: bool,
    cache: &str,
) -> Result<Response, ApiError> {
    let (file, fingerprint) = derivative_file(state, asset, &recipe).await?;
    let length = file.metadata().await?.len();
    Ok(Representation {
        tag: &format!("\"{fingerprint}\""),
        mime: recipe.format.mime(),
        cache,
        length,
        modified: UNIX_EPOCH + Duration::from_secs(asset.created_at as u64),
    }
    .serve(headers, head, true, |offset| async move {
        let mut file = file;
        if offset != 0 {
            file.seek(io::SeekFrom::Start(offset)).await?;
        }
        Ok(Box::new(file) as Reader)
    })
    .await)
}

async fn cached(path: &FsPath) -> io::Result<Option<File>> {
    match File::open(path).await {
        Ok(file) => Ok(Some(file)),
        Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(None),
        Err(e) => Err(e),
    }
}

async fn derivative_file(
    state: &State,
    asset: &Asset,
    recipe: &Recipe,
) -> Result<(File, String), ApiError> {
    let watermark = if let Some(id) = &recipe.wm {
        let mark = state.asset(id).await?;
        if mark.visibility != "public"
            || mark.size > 8 * 1024 * 1024
            || i64::from(mark.width) * i64::from(mark.height) > 4_194_304
        {
            return Err(ApiError::bad(
                "watermark must be a public asset with at most 4 MP and 8 MiB",
            ));
        }
        Some(mark)
    } else {
        None
    };
    let digest = Sha256::digest(format!(
        "{RECIPE}\n{}\n{}\n{}\n{}",
        state.encoder_version,
        asset.sha256,
        recipe.canonical(),
        watermark
            .as_ref()
            .map(|mark| mark.sha256.as_str())
            .unwrap_or_default()
    ));
    let fingerprint = hex::encode(digest);
    let path = state.cache.directory.join(&fingerprint);
    if let Some(file) = cached(&path).await? {
        state.cache.hits.fetch_add(1, Ordering::Relaxed);
        return Ok((file, fingerprint));
    }
    if !state.transform_rate.allow().await {
        return Err(ApiError::BUSY);
    }
    let _queue = state
        .transform_queue
        .try_acquire()
        .map_err(|_| ApiError::BUSY)?;
    let _lock = state.cache.locks[digest[0] as usize].lock().await;
    if let Some(file) = cached(&path).await? {
        state.cache.hits.fetch_add(1, Ordering::Relaxed);
        return Ok((file, fingerprint));
    }
    state.cache.misses.fetch_add(1, Ordering::Relaxed);
    let permit = state
        .cpu
        .clone()
        .try_acquire_owned()
        .map_err(|_| ApiError::BUSY)?;
    let source = materialize(state, &asset.storage_key).await?;
    let format = Format::from_mime(&asset.mime)?;
    let mark = match watermark {
        Some(mark) => Some((
            materialize(state, &mark.storage_key).await?,
            Format::from_mime(&mark.mime)?,
        )),
        None => None,
    };
    let recipe: Recipe = recipe.clone();
    let output = Temp::with_extension(&state.scratch, recipe.format.extension());
    let output = tokio::task::spawn_blocking(move || {
        let _permit = permit;
        recipe.render(
            source.path(),
            format,
            mark.as_ref().map(|(src, format)| (src.path(), *format)),
            output.path(),
        )?;
        Ok::<_, ApiError>(output)
    })
    .await
    .map_err(ApiError::internal)??;
    let file = File::open(output.path()).await?;
    let size = file.metadata().await?.len();
    state.cache.publish(output.path(), path, size).await?;
    Ok((file, fingerprint))
}

enum Source {
    Local(PathBuf),
    Staged(Temp),
}
impl Source {
    fn path(&self) -> &FsPath {
        match self {
            Self::Local(path) => path,
            Self::Staged(temp) => temp.path(),
        }
    }
}

async fn materialize(state: &State, key: &str) -> Result<Source, ApiError> {
    let key = Key::parse(key)?;
    if let Some(path) = state.storage.local_path(&key) {
        return Ok(Source::Local(path));
    }
    let temp = Temp::new(&state.scratch);
    tokio::io::copy(
        &mut state.storage.get_stream(&key, 0).await?,
        &mut temp.create().await?,
    )
    .await?;
    Ok(Source::Staged(temp))
}
