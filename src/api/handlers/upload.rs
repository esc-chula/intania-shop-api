use axum::{
    extract::{Multipart, State},
    http::StatusCode,
    response::IntoResponse,
    Json,
};
use serde::{Deserialize, Serialize};

use crate::api::ApiState;
use crate::api::handlers::product::response::ErrorResponse;

#[derive(Debug, Serialize)]
pub struct UploadResponse<T> {
    success: bool,
    data: T,
}

impl<T> UploadResponse<T> {
    pub fn new(data: T) -> Self {
        Self {
            success: true,
            data,
        }
    }
}

#[derive(Debug, Serialize, Deserialize)]
pub struct UploadData {
    pub uploads: Vec<FileUpload>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct FileUpload {
    pub url: String,
    pub object_name: String,
}

async fn upload_files_to_path(
    state: ApiState,
    mut multipart: Multipart,
    folder_path: &str,
) -> impl IntoResponse {
    let storage = &state.storage_service;
    let mut files: Vec<(Vec<u8>, String)> = Vec::new();

    while let Some(field) = multipart.next_field().await.unwrap_or(None) {
        let filename = field.file_name().unwrap_or("unnamed").to_string();
        let data = match field.bytes().await {
            Ok(bytes) => bytes.to_vec(),
            Err(e) => {
                return (
                    StatusCode::BAD_REQUEST,
                    Json(ErrorResponse::new(format!("Failed to read file: {}", e))),
                )
                    .into_response();
            }
        };
        files.push((data, filename));
    }

    if files.is_empty() {
        return (
            StatusCode::BAD_REQUEST,
            Json(ErrorResponse::new("No files provided".to_string())),
        )
            .into_response();
    }

    match storage.upload_files(files, folder_path).await {
        Ok(results) => {
            let uploads: Vec<FileUpload> = results
                .into_iter()
                .map(|(url, object_name)| FileUpload { url, object_name })
                .collect();

            (
                StatusCode::OK,
                Json(UploadResponse::new(UploadData { uploads })),
            )
                .into_response()
        }
        Err(e) => {
            eprintln!("Upload error: {:?}", e);
            (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(ErrorResponse::new(format!("Upload failed: {:#}", e))),
            )
                .into_response()
        }
    }
}

pub async fn upload_product_images(
    State(state): State<ApiState>,
    multipart: Multipart,
) -> impl IntoResponse {
    upload_files_to_path(state, multipart, "products/images").await
}

pub async fn upload_product_videos(
    State(state): State<ApiState>,
    multipart: Multipart,
) -> impl IntoResponse {
    upload_files_to_path(state, multipart, "products/videos").await
}
