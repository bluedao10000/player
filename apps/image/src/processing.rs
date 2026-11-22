use crate::ApiError;
use libvips::{bindings, ops, VipsApp, VipsImage};
use serde::{Deserialize, Serialize};
use std::ffi::{CStr, CString};
use std::path::Path;

pub const RECIPE: &str = "folio-v1";
pub const MAX_DIMENSION: u32 = 4096;

pub fn init(threads: i32) -> Result<String, libvips::error::Error> {
    let app = VipsApp::new("folio-image", false)?;
    app.concurrency_set(threads);
    app.cache_set_max(0);
    let version = app.version_string()?.to_owned();
    // libvips is process-wide and must outlive every blocking worker.
    std::mem::forget(app);
    Ok(version)
}

#[derive(Clone, Copy, Debug, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum Format {
    Jpeg,
    Png,
    Webp,
    Avif,
}

impl Format {
    pub fn detect(h: &[u8]) -> Option<Self> {
        if h.starts_with(b"\xff\xd8\xff") {
            Some(Self::Jpeg)
        } else if h.starts_with(b"\x89PNG\r\n\x1a\n") {
            Some(Self::Png)
        } else if h.starts_with(b"RIFF") && h.get(8..12) == Some(b"WEBP") {
            Some(Self::Webp)
        } else if h.get(4..12) == Some(b"ftypavif") {
            Some(Self::Avif)
        } else {
            None
        }
    }

    pub fn mime(self) -> &'static str {
        match self {
            Self::Jpeg => "image/jpeg",
            Self::Png => "image/png",
            Self::Webp => "image/webp",
            Self::Avif => "image/avif",
        }
    }

    pub fn from_mime(mime: &str) -> Result<Self, ApiError> {
        match mime {
            "image/jpeg" => Ok(Self::Jpeg),
            "image/png" => Ok(Self::Png),
            "image/webp" => Ok(Self::Webp),
            "image/avif" => Ok(Self::Avif),
            _ => Err(ApiError::internal("unknown stored MIME type")),
        }
    }

    fn load(self, filename: &str, sequential: bool) -> libvips::Result<VipsImage> {
        let expected: &[u8] = match self {
            Self::Jpeg => b"VipsForeignLoadJpegFile",
            Self::Png => b"VipsForeignLoadPngFile",
            Self::Webp => b"VipsForeignLoadWebpFile",
            Self::Avif => b"VipsForeignLoadHeifFile",
        };
        let name = CString::new(filename)
            .map_err(|_| libvips::error::Error::IOError("invalid image path"))?;
        // The returned class name belongs to libvips and is valid for the life of the process.
        let loader = unsafe {
            let ptr = bindings::vips_foreign_find_load(name.as_ptr());
            if ptr.is_null() {
                None
            } else {
                Some(CStr::from_ptr(ptr).to_bytes())
            }
        };
        if loader != Some(expected) {
            return Err(libvips::error::Error::IOError("unrecognized image loader"));
        }
        // Choose the normal loader by magic; do not pass output properties to a loader.
        let access = if sequential { "sequential" } else { "random" };
        VipsImage::new_from_file(&format!("{filename}[access={access},fail-on=error]"))
    }

    pub fn extension(self) -> &'static str {
        match self {
            Self::Jpeg => "jpg",
            Self::Png => "png",
            Self::Webp => "webp",
            Self::Avif => "avif",
        }
    }

    fn save(self, image: &VipsImage, path: &Path, q: u8) -> Result<(), ApiError> {
        let options = match self {
            Self::Jpeg => format!("Q={q},keep=none,background=255 255 255"),
            Self::Png => "compression=6,keep=none".into(),
            Self::Webp => format!("Q={q},effort=2,keep=none"),
            Self::Avif => format!("Q={q},effort=2,bitdepth=8,keep=none"),
        };
        image
            .image_write_to_file(&format!("{}[{options}]", path_str(path)?))
            .map_err(native_error)
    }
}

fn native_error(error: libvips::error::Error) -> ApiError {
    // Copy libvips's current-thread error text before clearing the buffer.
    let detail = unsafe {
        let ptr = bindings::vips_error_buffer();
        let detail = if ptr.is_null() {
            String::new()
        } else {
            CStr::from_ptr(ptr).to_string_lossy().into_owned()
        };
        bindings::vips_error_clear();
        detail
    };
    ApiError::internal(format_args!("{error}: {detail}"))
}

fn path_str(path: &Path) -> Result<&str, ApiError> {
    path.to_str()
        .ok_or_else(|| ApiError::internal("non-UTF8 storage path"))
}

pub fn inspect(path: &Path, format: Format, max_pixels: u64) -> Result<(i32, i32, i32), ApiError> {
    let invalid = |_| ApiError::bad("invalid or truncated image");
    let image = format.load(path_str(path)?, true).map_err(invalid)?;
    let (mut w, mut h) = (image.get_width(), image.get_height());
    if w <= 0
        || h <= 0
        || w > 16_384
        || h > 16_384
        || u64::from(w as u32) * u64::from(h as u32) > max_pixels
    {
        return Err(ApiError::bad("source dimensions exceed limit"));
    }
    if image.get_n_pages() > 1 {
        return Err(ApiError::bad(
            "animated or multi-page images are unsupported",
        ));
    }
    let orientation = image.get_orientation().clamp(1, 8);
    if orientation >= 5 {
        std::mem::swap(&mut w, &mut h);
    }
    ops::avg(&image).map_err(invalid)?;
    Ok((w, h, orientation))
}

#[derive(Clone, Copy, Deserialize, Serialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
enum Fit {
    Contain,
    Cover,
    Fill,
    Smart,
}

#[derive(Clone, Copy, Deserialize)]
#[serde(rename_all = "lowercase")]
enum Preset {
    Thumb,
    Reader,
}

#[derive(Default, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Query {
    w: Option<u32>,
    h: Option<u32>,
    fit: Option<Fit>,
    format: Option<Format>,
    q: Option<u8>,
    crop: Option<String>,
    preset: Option<Preset>,
    blur: Option<u8>,
    sharpen: Option<u8>,
    wm: Option<String>,
    #[serde(default)]
    pub exp: Option<u64>,
    #[serde(default)]
    pub sig: Option<String>,
}

#[derive(Clone, Serialize)]
pub struct Recipe {
    w: Option<u32>,
    h: Option<u32>,
    fit: Fit,
    pub format: Format,
    q: u8,
    crop: Option<[u32; 4]>,
    blur: u8,
    sharpen: u8,
    pub wm: Option<String>,
}

impl Query {
    pub fn recipe(
        &self,
        source: Format,
        width: u32,
        height: u32,
        max_pixels: u64,
    ) -> Result<Option<Recipe>, ApiError> {
        if self.w.is_none()
            && self.h.is_none()
            && self.fit.is_none()
            && self.format.is_none()
            && self.q.is_none()
            && self.crop.is_none()
            && self.preset.is_none()
            && self.blur.is_none()
            && self.sharpen.is_none()
            && self.wm.is_none()
        {
            return Ok(None);
        }
        let (w, h, fit, format) = match self.preset {
            Some(preset) => {
                if self.w.is_some() || self.h.is_some() || self.fit.is_some() {
                    return Err(ApiError::bad(
                        "preset and explicit dimensions are mutually exclusive",
                    ));
                }
                match preset {
                    Preset::Thumb => (Some(320), Some(320), Fit::Cover, Format::Webp),
                    Preset::Reader => (Some(1280), Some(1920), Fit::Contain, Format::Webp),
                }
            }
            None => (self.w, self.h, self.fit.unwrap_or(Fit::Contain), source),
        };
        let format = self.format.unwrap_or(format);
        let q = self
            .q
            .unwrap_or(if format == Format::Avif { 55 } else { 82 });
        if !(1..=95).contains(&q)
            || (format == Format::Avif && q > 75)
            || (format == Format::Png && self.q.is_some())
        {
            return Err(ApiError::bad("invalid encoding quality"));
        }
        let blur = self.blur.unwrap_or(0);
        let sharpen = self.sharpen.unwrap_or(0);
        if blur > 20 || sharpen > 5 {
            return Err(ApiError::bad("invalid effect strength"));
        }
        let crop: Option<[u32; 4]> = self
            .crop
            .as_ref()
            .map(|val| {
                val.split(',')
                    .map(str::parse)
                    .collect::<Result<Vec<_>, _>>()
                    .ok()
                    .and_then(|v| v.try_into().ok())
                    .ok_or(ApiError::bad("expected crop=x,y,w,h"))
            })
            .transpose()?;
        if let Some([x, y, cw, ch]) = crop {
            if cw == 0 || ch == 0 || x.saturating_add(cw) > width || y.saturating_add(ch) > height {
                return Err(ApiError::bad("crop is outside the image"));
            }
        }
        if (fit != Fit::Contain && (w.is_none() || h.is_none()))
            || [w, h]
                .into_iter()
                .flatten()
                .any(|d| !(1..=MAX_DIMENSION).contains(&d))
        {
            return Err(ApiError::bad("invalid transform dimensions"));
        }
        let (source_w, source_h) = crop.map(|[_, _, w, h]| (w, h)).unwrap_or((width, height));
        // Keep the omitted dimension bounded while preserving aspect ratio.
        let (max_w, max_h) = match (w, h) {
            (None, Some(h)) => (MAX_DIMENSION, h),
            (Some(w), None) => (w, MAX_DIMENSION),
            (Some(w), Some(h)) => (w, h),
            (None, None) => (source_w, source_h),
        };
        let (out_w, out_h) = if fit == Fit::Contain && (w.is_some() || h.is_some()) {
            let (sw, sh) = (u64::from(source_w), u64::from(source_h));
            let (mw, mh) = (u64::from(max_w), u64::from(max_h));
            if mw * sh <= mh * sw {
                (max_w, (mw * sh).div_ceil(sw) as u32)
            } else {
                ((mh * sw).div_ceil(sh) as u32, max_h)
            }
        } else {
            (max_w, max_h)
        };
        if out_w > MAX_DIMENSION
            || out_h > MAX_DIMENSION
            || u64::from(out_w) * u64::from(out_h) > max_pixels
            || (format == Format::Avif && u64::from(out_w) * u64::from(out_h) > 4_194_304)
        {
            return Err(ApiError::bad("output dimensions exceed limit"));
        }
        if let Some(id) = &self.wm {
            crate::assets::valid_id(id)?;
        }
        Ok(Some(Recipe {
            w,
            h,
            fit,
            format,
            q,
            crop,
            blur,
            sharpen,
            wm: self.wm.clone(),
        }))
    }
}

impl Recipe {
    pub fn canonical(&self) -> String {
        serde_json::to_string(self).expect("recipe is serializable")
    }

    pub fn render(
        &self,
        source: &Path,
        format: Format,
        watermark: Option<(&Path, Format)>,
        output: &Path,
    ) -> Result<(), ApiError> {
        let source = format
            .load(path_str(source)?, false)
            .map_err(ApiError::internal)?;
        let mut image = ops::autorot(&source).map_err(native_error)?;
        if let Some([x, y, w, h]) = self.crop {
            image = ops::extract_area(&image, x as i32, y as i32, w as i32, h as i32)
                .map_err(native_error)?;
        }
        if self.w.is_some() || self.h.is_some() {
            image = resize(
                &image,
                self.w.unwrap_or(MAX_DIMENSION) as i32,
                self.h.unwrap_or(MAX_DIMENSION) as i32,
                self.fit,
            )?;
        }
        if self.blur > 0 {
            image = ops::gaussblur(&image, f64::from(self.blur)).map_err(native_error)?;
        }
        if self.sharpen > 0 {
            image = ops::sharpen_with_opts(
                &image,
                &ops::SharpenOptions {
                    sigma: f64::from(self.sharpen),
                    ..Default::default()
                },
            )
            .map_err(native_error)?;
        }
        if let Some((path, format)) = watermark {
            let overlay = format.load(path_str(path)?, false).map_err(native_error)?;
            let overlay = ops::autorot(&overlay).map_err(native_error)?;
            let overlay = resize(
                &overlay,
                (image.get_width() / 4).max(1).min(overlay.get_width()),
                (image.get_height() / 4).max(1).min(overlay.get_height()),
                Fit::Contain,
            )?;
            image = ops::composite2_with_opts(
                &image,
                &overlay,
                ops::BlendMode::Over,
                &ops::Composite2Options {
                    x: image.get_width() - overlay.get_width(),
                    y: image.get_height() - overlay.get_height(),
                    ..Default::default()
                },
            )
            .map_err(native_error)?;
        }
        self.format.save(&image, output, self.q)
    }
}

// Crop before scaling so cover never creates an oversized intermediate for a narrow source.
fn resize(image: &VipsImage, width: i32, height: i32, fit: Fit) -> Result<VipsImage, ApiError> {
    let (w, h) = (image.get_width(), image.get_height());
    let cropped = match fit {
        Fit::Cover | Fit::Smart => {
            let crop_w = ((i64::from(h) * i64::from(width) / i64::from(height))
                .clamp(1, i64::from(w))) as i32;
            let crop_h = ((i64::from(w) * i64::from(height) / i64::from(width))
                .clamp(1, i64::from(h))) as i32;
            let crop = if fit == Fit::Smart && (crop_w != w || crop_h != h) {
                ops::smartcrop(image, crop_w, crop_h)
            } else {
                ops::extract_area(image, (w - crop_w) / 2, (h - crop_h) / 2, crop_w, crop_h)
            };
            Some(crop.map_err(native_error)?)
        }
        _ => None,
    };
    let image = cropped.as_ref().unwrap_or(image);
    let x = f64::from(width) / f64::from(image.get_width());
    let y = f64::from(height) / f64::from(image.get_height());
    if fit == Fit::Contain {
        ops::resize(image, x.min(y)).map_err(native_error)
    } else {
        ops::resize_with_opts(
            image,
            x,
            &ops::ResizeOptions {
                vscale: y,
                ..Default::default()
            },
        )
        .map_err(native_error)
    }
}
