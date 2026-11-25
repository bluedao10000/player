use crate::{
    delivery,
    processing::{self, Format},
    storage::{self, Key, Upload},
    ApiError, Shared, State,
};
use axum::{
    body::Body,
    extract::{Path, Query, State as ExtractState},
    http::{header, HeaderMap, StatusCode},
    response::{IntoResponse, Response},
    Json,
};
use futures_util::TryStreamExt;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sqlx::{sqlite::SqliteRow, Row, SqliteConnection};
use std::{io, time::Duration};
use tokio::fs::File;
use uuid::Uuid;

pub const FIELDS: &str = "id,storage_key,name,directory_id,mime,size,width,height,sha256,orientation,created_at,source,metadata,visibility,previous_id,source_asset_id";

#[derive(Serialize, sqlx::FromRow)]
pub struct Asset {
    pub id: String,
    pub storage_key: String,
    pub name: String,
    pub directory_id: Option<String>,
    pub mime: String,
    pub size: i64,
    pub width: i32,
    pub height: i32,
    pub sha256: String,
    pub orientation: i32,
    pub created_at: i64,
    pub source: String,
    #[sqlx(json)]
    pub metadata: Value,
    pub visibility: String,
    pub previous_id: Option<String>,
    pub source_asset_id: Option<String>,
}

impl State {
    pub async fn public_image(&self, digest: &str) -> Result<Asset, ApiError> {
        sqlx::query_as(&format!(
            "SELECT {FIELDS} FROM assets
            WHERE sha256=? AND visibility='public' AND deleted_at IS NULL ORDER BY rowid LIMIT 1"
        ))
        .bind(digest)
        .fetch_optional(&self.db)
        .await?
        .ok_or(ApiError::NOT_FOUND)
    }

    pub async fn asset(&self, id: &str) -> Result<Asset, ApiError> {
        valid_id(id)?;
        sqlx::query_as(&format!(
            "SELECT {FIELDS} FROM assets WHERE id=? AND deleted_at IS NULL"
        ))
        .bind(id)
        .fetch_optional(&self.db)
        .await?
        .ok_or(ApiError::NOT_FOUND)
    }
}

pub fn valid_id(id: &str) -> Result<(), ApiError> {
    if Uuid::parse_str(id).is_ok_and(|uuid| uuid.to_string() == id) {
        Ok(())
    } else {
        Err(ApiError::NOT_FOUND)
    }
}

fn name(value: &str) -> Result<String, ApiError> {
    let value = value.trim();
    if value.is_empty()
        || value.len() > 240
        || [".", ".."].contains(&value)
        || value
            .chars()
            .any(|c| c.is_control() || matches!(c, '/' | '\\'))
    {
        return Err(ApiError::bad("invalid name"));
    }
    Ok(value.into())
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct UploadArgs {
    pub name: String,
    pub directory_id: Option<String>,
    pub previous_id: Option<String>,
    pub source: Option<String>,
}

pub struct Prepared {
    key: Key,
    format: Format,
    size: u64,
    width: i32,
    height: i32,
    orientation: i32,
}

pub async fn prepare(
    state: &Shared,
    input: Upload,
    content_type: Option<&str>,
) -> Result<Prepared, ApiError> {
    prepare_with_permit(state, input, content_type, None).await
}

pub async fn prepare_with_permit(
    state: &Shared,
    input: Upload,
    content_type: Option<&str>,
    permit: Option<tokio::sync::OwnedSemaphorePermit>,
) -> Result<Prepared, ApiError> {
    let (temp, key, size, header) = storage::stage(&state.scratch, input, state.max_upload)
        .await
        .map_err(|e| match e.kind() {
            io::ErrorKind::FileTooLarge => {
                ApiError(StatusCode::PAYLOAD_TOO_LARGE, "upload too large")
            }
            io::ErrorKind::InvalidData => ApiError::bad("invalid upload stream"),
            _ => e.into(),
        })?;
    let format = Format::detect(&header).ok_or(ApiError(
        StatusCode::UNSUPPORTED_MEDIA_TYPE,
        "unsupported image",
    ))?;
    if content_type.is_some_and(|mime| mime != "application/octet-stream" && mime != format.mime())
    {
        return Err(ApiError(
            StatusCode::UNSUPPORTED_MEDIA_TYPE,
            "Content-Type does not match image",
        ));
    }
    let permit = match permit {
        Some(permit) => permit,
        None => state
            .cpu
            .clone()
            .try_acquire_owned()
            .map_err(|_| ApiError::BUSY)?,
    };
    let pixels = state.max_pixels;
    let (temp, (width, height, orientation)) = tokio::task::spawn_blocking(move || {
        let _permit = permit;
        let dimensions = processing::inspect(temp.path(), format, pixels)?;
        Ok::<_, ApiError>((temp, dimensions))
    })
    .await
    .map_err(ApiError::internal)??;
    state
        .storage
        .put_stream(&key, Box::new(File::open(temp.path()).await?))
        .await?;
    Ok(Prepared {
        key,
        format,
        size,
        width,
        height,
        orientation,
    })
}

impl Prepared {
    pub async fn insert(
        self,
        conn: &mut SqliteConnection,
        id: &str,
        args: &UploadArgs,
        source_asset: Option<&str>,
    ) -> Result<Asset, ApiError> {
        let result = sqlx::query_as(&format!("INSERT INTO assets
            (id,storage_key,name,directory_id,mime,size,width,height,sha256,orientation,created_at,source,previous_id,source_asset_id)
            SELECT ?,?,?,?,?,?,?,?,?,?,?,?,?,?
            WHERE ? IS NULL OR EXISTS(SELECT 1 FROM assets WHERE id=? AND deleted_at IS NULL)
            RETURNING {FIELDS}"))
            .bind(id).bind(self.key.as_str()).bind(&args.name).bind(&args.directory_id)
            .bind(self.format.mime()).bind(self.size as i64).bind(self.width).bind(self.height)
            .bind(self.key.as_str()).bind(self.orientation).bind(crate::now() as i64)
            .bind(args.source.as_deref().unwrap_or("upload")).bind(&args.previous_id).bind(source_asset)
            .bind(&args.previous_id).bind(&args.previous_id)
            .fetch_optional(conn).await.map_err(ApiError::constraint)?;
        result.ok_or(ApiError::NOT_FOUND)
    }
}

pub async fn upload(
    ExtractState(state): ExtractState<Shared>,
    Query(mut args): Query<UploadArgs>,
    headers: HeaderMap,
    body: Body,
) -> Result<impl IntoResponse, ApiError> {
    args.name = name(&args.name)?;
    if let Some(id) = &args.previous_id {
        state.asset(id).await?;
    }
    if args
        .source
        .as_ref()
        .is_some_and(|v| v.len() > 240 || v.chars().any(char::is_control))
    {
        return Err(ApiError::bad("invalid source"));
    }
    if headers
        .get(header::CONTENT_LENGTH)
        .and_then(|value| value.to_str().ok())
        .and_then(|value| value.parse::<u64>().ok())
        .is_some_and(|length| length > state.max_upload)
    {
        return Err(ApiError(StatusCode::PAYLOAD_TOO_LARGE, "upload too large"));
    }
    let content_type = headers
        .get(header::CONTENT_TYPE)
        .map(|value| {
            value
                .to_str()
                .map_err(|_| ApiError(StatusCode::UNSUPPORTED_MEDIA_TYPE, "invalid Content-Type"))
        })
        .transpose()?;
    let _permit = state.uploads.try_acquire().map_err(|_| ApiError::BUSY)?;
    let stream = body
        .into_data_stream()
        .map_err(|e| io::Error::new(io::ErrorKind::InvalidData, e));
    let prepared = tokio::time::timeout(
        Duration::from_secs(120),
        prepare(&state, Box::pin(stream), content_type),
    )
    .await
    .map_err(|_| ApiError(StatusCode::REQUEST_TIMEOUT, "upload timed out"))??;
    let asset = prepared
        .insert(
            &mut *state.db.acquire().await?,
            &Uuid::new_v4().to_string(),
            &args,
            None,
        )
        .await?;
    Ok((StatusCode::CREATED, Json(asset)))
}

pub async fn detail(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
) -> Result<Json<Asset>, ApiError> {
    Ok(Json(state.asset(&id).await?))
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ListArgs {
    directory_id: Option<String>,
    q: Option<String>,
    after: Option<i64>,
    limit: Option<u32>,
    previous_id: Option<String>,
    source_asset_id: Option<String>,
}

pub async fn list(
    ExtractState(state): ExtractState<Shared>,
    Query(args): Query<ListArgs>,
) -> Result<Json<Value>, ApiError> {
    let limit = args.limit.unwrap_or(50);
    if !(1..=100).contains(&limit)
        || args
            .q
            .as_ref()
            .is_some_and(|q| q.len() > 128 || q.chars().any(char::is_control))
    {
        return Err(ApiError::bad("invalid pagination or search"));
    }
    let mut query = sqlx::QueryBuilder::new(format!(
        "SELECT rowid AS cursor,{FIELDS} FROM assets WHERE deleted_at IS NULL AND rowid > "
    ));
    query.push_bind(args.after.unwrap_or(0));
    if let Some(directory) = args.directory_id {
        if directory == "root" {
            query.push(" AND directory_id IS NULL");
        } else {
            valid_id(&directory)?;
            query.push(" AND directory_id = ").push_bind(directory);
        }
    }
    for (column, value) in [
        ("previous_id", args.previous_id),
        ("source_asset_id", args.source_asset_id),
    ] {
        if let Some(id) = value {
            valid_id(&id)?;
            query.push(" AND ").push(column).push(" = ").push_bind(id);
        }
    }
    if let Some(search) = args.q.filter(|q| !q.is_empty()) {
        if search.chars().count() >= 3 {
            // MATCH takes a quoted literal, so the caller cannot invoke FTS query operators.
            let literal = format!("\"{}\"", search.replace('"', "\"\""));
            query
                .push(" AND rowid IN (SELECT rowid FROM asset_search WHERE asset_search MATCH ")
                .push_bind(literal)
                .push(")");
        }
        query
            .push(" AND (instr(lower(name), lower(")
            .push_bind(search.clone())
            .push(")) > 0 OR instr(lower(metadata), lower(")
            .push_bind(search)
            .push(")) > 0)");
    }
    query.push(" ORDER BY rowid LIMIT ").push_bind(limit + 1);
    let rows = query.build().fetch_all(&state.db).await?;
    let next = if rows.len() > limit as usize {
        Some(rows[limit as usize - 1].get::<i64, _>("cursor"))
    } else {
        None
    };
    let items = rows
        .iter()
        .take(limit as usize)
        .map(|row: &SqliteRow| sqlx::FromRow::from_row(row))
        .collect::<Result<Vec<Asset>, _>>()?;
    Ok(Json(json!({"items":items,"next":next})))
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Edit {
    name: Option<String>,
    metadata: Option<Value>,
}

pub async fn edit(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
    Json(edit): Json<Edit>,
) -> Result<Json<Asset>, ApiError> {
    let name = edit.name.as_deref().map(name).transpose()?;
    let metadata = edit
        .metadata
        .map(|value| {
            let text = value.to_string();
            if !value.is_object() || text.len() > 16_384 {
                return Err(ApiError::bad("invalid metadata"));
            }
            Ok(text)
        })
        .transpose()?;
    let asset = sqlx::query_as(&format!(
        "UPDATE assets SET name=coalesce(?,name),metadata=coalesce(?,metadata)
        WHERE id=? AND deleted_at IS NULL RETURNING {FIELDS}"
    ))
    .bind(name)
    .bind(metadata)
    .bind(id)
    .fetch_optional(&state.db)
    .await?
    .ok_or(ApiError::NOT_FOUND)?;
    Ok(Json(asset))
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Move {
    directory_id: Option<String>,
}

pub async fn move_asset(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
    Json(target): Json<Move>,
) -> Result<Json<Asset>, ApiError> {
    let asset = sqlx::query_as(&format!(
        "UPDATE assets SET directory_id=? WHERE id=? AND deleted_at IS NULL RETURNING {FIELDS}"
    ))
    .bind(target.directory_id)
    .bind(id)
    .fetch_optional(&state.db)
    .await
    .map_err(ApiError::constraint)?
    .ok_or(ApiError::NOT_FOUND)?;
    Ok(Json(asset))
}

pub async fn delete(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
) -> Result<StatusCode, ApiError> {
    let mut tx = state.db.begin_with("BEGIN IMMEDIATE").await?;
    let row = sqlx::query_as::<_, (String, Option<i64>)>(
        "SELECT visibility,deleted_at FROM assets WHERE id=?",
    )
    .bind(&id)
    .fetch_optional(&mut *tx)
    .await?
    .ok_or(ApiError::NOT_FOUND)?;
    if row.0 == "public"
        || sqlx::query_scalar::<_, bool>(
            "SELECT
        EXISTS(SELECT 1 FROM published_refs WHERE asset_id=?) OR
        EXISTS(SELECT 1 FROM jobs WHERE asset_id=? AND state IN ('queued','running')) OR
        EXISTS(SELECT 1 FROM jobs WHERE mask_id=? AND state IN ('queued','running'))",
        )
        .bind(&id)
        .bind(&id)
        .bind(&id)
        .fetch_one(&mut *tx)
        .await?
    {
        return Err(ApiError::conflict("asset is public, referenced or in use"));
    }
    if row.1.is_none() {
        sqlx::query("UPDATE assets SET deleted_at=?, directory_id=NULL WHERE id=?")
            .bind(crate::now() as i64)
            .bind(id)
            .execute(&mut *tx)
            .await?;
    }
    tx.commit().await?;
    Ok(StatusCode::NO_CONTENT)
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Publish {
    reference: String,
}

pub async fn publish(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
    Json(args): Json<Publish>,
) -> Result<Json<Value>, ApiError> {
    if args.reference.trim().is_empty()
        || args.reference.len() > 512
        || args.reference.chars().any(char::is_control)
    {
        return Err(ApiError::bad("invalid publication reference"));
    }
    let mut tx = state.db.begin_with("BEGIN IMMEDIATE").await?;
    let updated =
        sqlx::query("UPDATE assets SET visibility='public' WHERE id=? AND deleted_at IS NULL")
            .bind(&id)
            .execute(&mut *tx)
            .await?;
    if updated.rows_affected() != 1 {
        return Err(ApiError::NOT_FOUND);
    }
    sqlx::query(
        "INSERT INTO published_refs(asset_id,reference) VALUES(?,?) ON CONFLICT DO NOTHING",
    )
    .bind(&id)
    .bind(args.reference)
    .execute(&mut *tx)
    .await?;
    tx.commit().await?;
    Ok(Json(json!({"url": format!("/i/{id}")})))
}

#[derive(Deserialize, Serialize, sqlx::FromRow)]
#[serde(deny_unknown_fields)]
pub struct Directory {
    id: Option<String>,
    parent_id: Option<String>,
    name: String,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct DirectoryArgs {
    name: String,
    parent_id: Option<String>,
}

pub async fn create_directory(
    ExtractState(state): ExtractState<Shared>,
    Json(args): Json<DirectoryArgs>,
) -> Result<impl IntoResponse, ApiError> {
    let directory = sqlx::query_as::<_, Directory>(
        "INSERT INTO directories(id,parent_id,name) VALUES(?,?,?) RETURNING id,parent_id,name",
    )
    .bind(Uuid::new_v4().to_string())
    .bind(args.parent_id)
    .bind(name(&args.name)?)
    .fetch_one(&state.db)
    .await
    .map_err(ApiError::constraint)?;
    Ok((StatusCode::CREATED, Json(directory)))
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct DirectoryList {
    after: Option<String>,
    parent_id: Option<String>,
}

pub async fn list_directories(
    ExtractState(state): ExtractState<Shared>,
    Query(args): Query<DirectoryList>,
) -> Result<Json<Value>, ApiError> {
    let after = args.after.unwrap_or_default();
    let mut query =
        sqlx::QueryBuilder::new("SELECT id,parent_id,name FROM directories WHERE id > ");
    query.push_bind(after);
    if let Some(parent) = args.parent_id {
        if parent == "root" {
            query.push(" AND parent_id IS NULL");
        } else {
            query.push(" AND parent_id = ").push_bind(parent);
        }
    }
    let mut items = query
        .push(" ORDER BY id LIMIT 101")
        .build_query_as::<Directory>()
        .fetch_all(&state.db)
        .await?;
    let next = if items.len() > 100 {
        items.pop();
        items.last().and_then(|item| item.id.clone())
    } else {
        None
    };
    Ok(Json(json!({"items":items,"next":next})))
}

pub async fn delete_directory(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
) -> Result<StatusCode, ApiError> {
    let mut tx = state.db.begin_with("BEGIN IMMEDIATE").await?;
    let nonempty: bool = sqlx::query_scalar(
        "SELECT EXISTS(SELECT 1 FROM assets WHERE directory_id=? AND deleted_at IS NULL)
        OR EXISTS(SELECT 1 FROM directories WHERE parent_id=?)",
    )
    .bind(&id)
    .bind(&id)
    .fetch_one(&mut *tx)
    .await?;
    if nonempty {
        return Err(ApiError::conflict("directory is not empty"));
    }
    let result = sqlx::query("DELETE FROM directories WHERE id=?")
        .bind(id)
        .execute(&mut *tx)
        .await?;
    tx.commit().await?;
    if result.rows_affected() == 0 {
        return Err(ApiError::NOT_FOUND);
    }
    Ok(StatusCode::NO_CONTENT)
}

pub async fn download(
    ExtractState(state): ExtractState<Shared>,
    Path(id): Path<String>,
    method: axum::http::Method,
    headers: HeaderMap,
) -> Result<Response, ApiError> {
    let asset = state.asset(&id).await?;
    let mut response = crate::images::original(
        &state,
        &asset,
        &headers,
        method == axum::http::Method::HEAD,
        delivery::PRIVATE,
    )
    .await?;
    if response.status().is_success() {
        let encoded: String = asset
            .name
            .bytes()
            .map(|b| {
                if b.is_ascii_alphanumeric() || b"-_.".contains(&b) {
                    (b as char).to_string()
                } else {
                    format!("%{b:02X}")
                }
            })
            .collect();
        response.headers_mut().insert(
            header::CONTENT_DISPOSITION,
            format!("attachment; filename*=UTF-8''{encoded}")
                .parse()
                .unwrap(),
        );
    }
    Ok(response)
}

pub fn bytes_input(bytes: Vec<u8>) -> Upload {
    Box::pin(futures_util::stream::once(async move { Ok(bytes.into()) }))
}
