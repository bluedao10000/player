use axum::body::Bytes;
use folio_image::storage::{Key, LocalStorage, ObjectStorage};
use sha2::{Digest, Sha256};
use std::io;

#[tokio::test]
async fn checksums_keys_and_atomic_concurrent_put() {
    let path = tempfile::tempdir().unwrap();
    let storage = LocalStorage::open(path.path(), path.path()).await.unwrap();
    let bytes = Bytes::from_static(b"immutable object");
    let hash = hex::encode(Sha256::digest(&bytes));
    let key = Key::parse(&hash).unwrap();
    assert!(Key::parse("../../etc/passwd").is_err());
    assert!(Key::parse(&hash.to_ascii_uppercase()).is_err());
    assert!(!storage.exists(&key).await.unwrap());
    assert_eq!(
        storage
            .put(&key, Bytes::from_static(b"wrong"))
            .await
            .unwrap_err()
            .kind(),
        io::ErrorKind::InvalidData
    );
    let (a, b) = tokio::join!(
        storage.put(&key, bytes.clone()),
        storage.put(&key, bytes.clone())
    );
    a.unwrap();
    b.unwrap();
    assert_eq!(storage.get(&key).await.unwrap(), bytes);
    assert_eq!(storage.head(&key).await.unwrap(), bytes.len() as u64);
    assert_eq!(
        storage
            .list(u8::from_str_radix(&hash[..2], 16).unwrap())
            .await
            .unwrap(),
        std::slice::from_ref(&key)
    );
    storage.delete(&key).await.unwrap();
    assert!(!storage.exists(&key).await.unwrap());
}
