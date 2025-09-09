use tower_http::cors::CorsLayer;
use axum::http::{Method, HeaderName};

pub fn layer() -> CorsLayer {
    CorsLayer::new()
        .allow_methods([Method::GET, Method::POST, Method::PUT, Method::DELETE, Method::OPTIONS])
        .allow_headers([
            HeaderName::from_static("content-type"),
            HeaderName::from_static("authorization"),
            HeaderName::from_static("accept"),
            HeaderName::from_static("origin"),
            HeaderName::from_static("user-agent"),
        ])
        .allow_origin([
            "http://localhost:3000".parse().unwrap(),
            "http://localhost:8080".parse().unwrap(),
            "http://127.0.0.1:3000".parse().unwrap(),
            "http://127.0.0.1:8080".parse().unwrap(),
        ])
        .allow_credentials(true)
}
