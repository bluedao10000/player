use crate::storage::Reader;
use axum::{
    body::Body,
    http::{header, HeaderMap, HeaderValue, StatusCode},
    response::{IntoResponse, Response},
};
use std::{future::Future, io, time::SystemTime};
use tokio::io::AsyncReadExt;
use tokio_util::io::ReaderStream;

pub const PRIVATE: &str = "private, no-store";
pub const IMMUTABLE: &str = "public, max-age=31536000, immutable";

pub fn error(status: StatusCode) -> Response {
    (status, [(header::CACHE_CONTROL, PRIVATE)]).into_response()
}

pub struct Representation<'a> {
    pub tag: &'a str,
    pub mime: &'a str,
    pub cache: &'a str,
    pub length: u64,
    pub modified: SystemTime,
}

impl Representation<'_> {
    pub async fn serve<F, Fut>(
        &self,
        headers: &HeaderMap,
        head: bool,
        conditional: bool,
        open: F,
    ) -> Response
    where
        F: FnOnce(u64) -> Fut,
        Fut: Future<Output = io::Result<Reader>>,
    {
        let last_modified = httpdate::fmt_http_date(self.modified);
        let unchanged = conditional
            && headers.get_all(header::IF_NONE_MATCH).iter().any(|value| {
                value.to_str().is_ok_and(|tags| {
                    tags.split(',').any(|tag| {
                        let tag = tag.trim();
                        tag == "*" || tag.strip_prefix("W/").unwrap_or(tag) == self.tag
                    })
                })
            });
        let if_range = headers
            .get(header::IF_RANGE)
            .and_then(|value| value.to_str().ok());
        let mut response = if unchanged {
            StatusCode::NOT_MODIFIED.into_response()
        } else {
            let range_header = headers
                .get(header::RANGE)
                .and_then(|value| value.to_str().ok())
                .filter(|_| {
                    !head
                        && if_range.is_none_or(|value| value == self.tag || value == last_modified)
                });
            let (status, start, count) =
                match range_header.and_then(|value| value.strip_prefix("bytes=")) {
                    Some(spec) => match range(spec, self.length) {
                        Some((start, end)) => (StatusCode::PARTIAL_CONTENT, start, end - start + 1),
                        None => {
                            let mut response = error(StatusCode::RANGE_NOT_SATISFIABLE);
                            response.headers_mut().insert(
                                header::CONTENT_RANGE,
                                hv(&format!("bytes */{}", self.length)),
                            );
                            return response;
                        }
                    },
                    None => (StatusCode::OK, 0, self.length),
                };
            let mut response = if head {
                status.into_response()
            } else {
                match open(start).await {
                    Ok(reader) => (
                        status,
                        Body::from_stream(ReaderStream::with_capacity(
                            reader.take(count),
                            64 * 1024,
                        )),
                    )
                        .into_response(),
                    Err(e) => {
                        tracing::warn!(error = %e, "open image");
                        return error(StatusCode::SERVICE_UNAVAILABLE);
                    }
                }
            };
            response
                .headers_mut()
                .insert(header::CONTENT_LENGTH, hv(&count.to_string()));
            if status == StatusCode::PARTIAL_CONTENT {
                response.headers_mut().insert(
                    header::CONTENT_RANGE,
                    hv(&format!(
                        "bytes {start}-{}/{}",
                        start + count - 1,
                        self.length
                    )),
                );
            }
            response
        };
        let out = response.headers_mut();
        out.insert(header::ETAG, hv(self.tag));
        out.insert(header::LAST_MODIFIED, hv(&last_modified));
        out.insert(header::CACHE_CONTROL, hv(self.cache));
        out.insert(header::CONTENT_TYPE, hv(self.mime));
        out.insert(header::ACCEPT_RANGES, HeaderValue::from_static("bytes"));
        out.insert(
            header::X_CONTENT_TYPE_OPTIONS,
            HeaderValue::from_static("nosniff"),
        );
        response
    }
}

fn hv(value: &str) -> HeaderValue {
    HeaderValue::from_str(value).expect("validated response header")
}

fn range(spec: &str, length: u64) -> Option<(u64, u64)> {
    let (left, right) = spec.split_once('-')?;
    let last = length.checked_sub(1)?;
    if left.is_empty() {
        let suffix = right.parse::<u64>().ok()?.checked_sub(1)?;
        return Some((last.saturating_sub(suffix), last));
    }
    let start = left.parse::<u64>().ok()?;
    let end = if right.is_empty() {
        last
    } else {
        right.parse::<u64>().ok()?.min(last)
    };
    (start <= end).then_some((start, end))
}
