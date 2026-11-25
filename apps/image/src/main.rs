use axum::serve::ListenerExt;
use folio_image::{service, Config};
use std::env;
use tokio::{net::TcpListener, signal};

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "folio_image=info".into()),
        )
        .init();
    let config = Config::from_env(env::var("IMAGE_DATA_DIR")?.into()).await?;
    let service = service(config).await?;
    let bind = env::var("IMAGE_BIND").unwrap_or_else(|_| "127.0.0.1:14580".into());
    let listener = TcpListener::bind(&bind).await?;
    tracing::info!(address = %listener.local_addr()?, "image service listening");
    let listener = listener.tap_io(|stream| {
        if let Err(error) = stream.set_nodelay(true) {
            tracing::debug!(%error, "could not disable Nagle for image delivery");
        }
    });
    axum::serve(listener, service.router.clone())
        .with_graceful_shutdown(shutdown())
        .await?;
    Ok(())
}

async fn shutdown() {
    #[cfg(unix)]
    {
        let mut term =
            signal::unix::signal(signal::unix::SignalKind::terminate()).expect("SIGTERM handler");
        tokio::select! {
            _ = signal::ctrl_c() => {},
            _ = term.recv() => {},
        }
    }
    #[cfg(not(unix))]
    let _ = signal::ctrl_c().await;
}
