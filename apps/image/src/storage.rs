use async_trait::async_trait;
use axum::body::Bytes;
use futures_util::{Stream, StreamExt};
use sha2::{Digest, Sha256};
use std::{
    io,
    path::{Path, PathBuf},
    pin::Pin,
};
use tokio::{
    fs::{self, File, OpenOptions},
    io::{AsyncRead, AsyncReadExt, AsyncSeekExt, AsyncWriteExt},
};
use uuid::Uuid;

pub type Reader = Box<dyn AsyncRead + Send + Unpin>;
pub type Upload = Pin<Box<dyn Stream<Item = io::Result<Bytes>> + Send>>;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Key(String);

impl Key {
    pub fn parse(value: &str) -> io::Result<Self> {
        if value.len() != 64
            || !value
                .bytes()
                .all(|b| b.is_ascii_hexdigit() && !b.is_ascii_uppercase())
        {
            return Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                "invalid object key",
            ));
        }
        Ok(Self(value.into()))
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }
}

#[async_trait]
pub trait ObjectStorage: Send + Sync {
    async fn put(&self, key: &Key, bytes: Bytes) -> io::Result<()> {
        self.put_stream(key, Box::new(std::io::Cursor::new(bytes)))
            .await
    }
    async fn put_stream(&self, key: &Key, reader: Reader) -> io::Result<()>;
    async fn get(&self, key: &Key) -> io::Result<Vec<u8>> {
        let mut bytes = Vec::new();
        self.get_stream(key, 0)
            .await?
            .read_to_end(&mut bytes)
            .await?;
        Ok(bytes)
    }
    async fn get_stream(&self, key: &Key, offset: u64) -> io::Result<Reader>;
    async fn head(&self, key: &Key) -> io::Result<u64>;
    async fn delete(&self, key: &Key) -> io::Result<()>;
    async fn exists(&self, key: &Key) -> io::Result<bool> {
        match self.head(key).await {
            Ok(_) => Ok(true),
            Err(e) if e.kind() == io::ErrorKind::NotFound => Ok(false),
            Err(e) => Err(e),
        }
    }
    // Walk a single hash prefix so maintenance does not build an unbounded inventory.
    async fn list(&self, prefix: u8) -> io::Result<Vec<Key>>;
    fn local_path(&self, _key: &Key) -> Option<PathBuf> {
        None
    }
}

pub struct LocalStorage {
    objects: PathBuf,
    scratch: PathBuf,
}

impl LocalStorage {
    pub async fn open(root: &Path, scratch: &Path) -> io::Result<Self> {
        let objects = root.join("objects");
        fs::create_dir_all(&objects).await?;
        Ok(Self {
            objects,
            scratch: scratch.into(),
        })
    }

    fn path(&self, key: &Key) -> PathBuf {
        self.objects
            .join(&key.0[..2])
            .join(&key.0[2..4])
            .join(&key.0)
    }
}

#[async_trait]
impl ObjectStorage for LocalStorage {
    async fn put_stream(&self, key: &Key, mut reader: Reader) -> io::Result<()> {
        let path = self.path(key);
        let parent = path.parent().unwrap();
        let temp = Temp::new(&self.scratch);
        let mut file = temp.create().await?;
        let mut buf = vec![0; 64 * 1024];
        let mut digest = Sha256::new();
        let mut size = 0;
        loop {
            let n = reader.read(&mut buf).await?;
            if n == 0 {
                break;
            }
            digest.update(&buf[..n]);
            file.write_all(&buf[..n]).await?;
            size += n as u64;
        }
        if hex::encode(digest.finalize()) != key.0 {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                "object checksum mismatch",
            ));
        }
        file.sync_all().await?;
        fs::create_dir_all(parent).await?;
        match fs::hard_link(temp.path(), &path).await {
            Ok(()) => {}
            Err(e) if e.kind() == io::ErrorKind::AlreadyExists && self.head(key).await? == size => {
            }
            Err(e) => return Err(e),
        }
        // Make all newly created directory entries durable before the SQLite commit.
        for directory in [parent, parent.parent().unwrap(), &self.objects] {
            File::open(directory).await?.sync_all().await?;
        }
        Ok(())
    }

    async fn get_stream(&self, key: &Key, offset: u64) -> io::Result<Reader> {
        let mut file = File::open(self.path(key)).await?;
        if offset != 0 {
            file.seek(io::SeekFrom::Start(offset)).await?;
        }
        Ok(Box::new(file))
    }

    async fn head(&self, key: &Key) -> io::Result<u64> {
        Ok(fs::metadata(self.path(key)).await?.len())
    }

    async fn delete(&self, key: &Key) -> io::Result<()> {
        fs::remove_file(self.path(key)).await
    }

    async fn list(&self, prefix: u8) -> io::Result<Vec<Key>> {
        let mut keys = Vec::new();
        let prefix = format!("{prefix:02x}");
        let mut dirs = match fs::read_dir(self.objects.join(&prefix)).await {
            Ok(dirs) => dirs,
            Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(keys),
            Err(e) => return Err(e),
        };
        while let Some(dir) = dirs.next_entry().await? {
            if !dir.file_type().await?.is_dir() {
                continue;
            }
            let mut files = fs::read_dir(dir.path()).await?;
            while let Some(file) = files.next_entry().await? {
                if file.file_type().await?.is_file() {
                    if let Some(key) = file.file_name().to_str().and_then(|v| Key::parse(v).ok()) {
                        if key.0.starts_with(&prefix) && self.path(&key) == file.path() {
                            keys.push(key);
                        }
                    }
                }
            }
        }
        keys.sort_unstable_by(|left, right| left.0.cmp(&right.0));
        Ok(keys)
    }

    fn local_path(&self, key: &Key) -> Option<PathBuf> {
        Some(self.path(key))
    }
}

pub struct Temp(PathBuf);

impl Temp {
    pub fn new(directory: &Path) -> Self {
        Self::with_extension(directory, "tmp")
    }
    pub fn with_extension(directory: &Path, extension: &str) -> Self {
        Self(directory.join(format!("{}.{}", Uuid::new_v4(), extension)))
    }
    pub fn path(&self) -> &Path {
        &self.0
    }
    pub async fn create(&self) -> io::Result<File> {
        OpenOptions::new()
            .write(true)
            .create_new(true)
            .open(&self.0)
            .await
    }
}

impl Drop for Temp {
    fn drop(&mut self) {
        if let Ok(runtime) = tokio::runtime::Handle::try_current() {
            let path = self.0.clone();
            runtime.spawn(async move {
                let _ = fs::remove_file(path).await;
            });
        }
    }
}

pub async fn stage(
    directory: &Path,
    mut stream: Upload,
    max: u64,
) -> io::Result<(Temp, Key, u64, Vec<u8>)> {
    let temp = Temp::new(directory);
    let mut file = temp.create().await?;
    let mut digest = Sha256::new();
    let mut size = 0u64;
    let mut header = Vec::with_capacity(32);
    while let Some(chunk) = stream.next().await {
        let chunk = chunk?;
        size = size.saturating_add(chunk.len() as u64);
        if size > max {
            return Err(io::Error::new(
                io::ErrorKind::FileTooLarge,
                "upload too large",
            ));
        }
        header.extend_from_slice(&chunk[..chunk.len().min(32 - header.len())]);
        digest.update(&chunk);
        file.write_all(&chunk).await?;
    }
    file.flush().await?;
    Ok((temp, Key(hex::encode(digest.finalize())), size, header))
}
