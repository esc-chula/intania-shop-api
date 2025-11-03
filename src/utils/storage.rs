use anyhow::{Context, Result};
use google_cloud_storage::client::Storage;
use std::path::Path;
use uuid::Uuid;

#[derive(Clone)]
pub struct StorageService {
    client: Storage,
    bucket_name: String,
}

impl StorageService {
    pub async fn new(bucket_name: String) -> Result<Self> {
        let client = Storage::builder()
            .build()
            .await
            .context("Failed to create Google Cloud Storage client")?;

        let full_bucket_name = format!("projects/_/buckets/{}", bucket_name);

        Ok(Self {
            client,
            bucket_name: full_bucket_name,
        })
    }

    pub async fn upload_file(
        &self,
        file_data: Vec<u8>,
        original_filename: &str,
        folder: &str, 
    ) -> Result<(String, String)> {
        let extension = Path::new(original_filename)
            .extension()
            .and_then(|s| s.to_str())
            .unwrap_or("bin");
        
        let sanitized_name = sanitize_filename(original_filename);
        let object_name = format!("{}/{}-{}.{}", folder, Uuid::new_v4(), sanitized_name, extension);

        let mime_type = mime_guess::from_path(original_filename)
            .first_or_octet_stream()
            .to_string();

        use bytes::Bytes;
        let bytes_data = Bytes::from(file_data);
        
        let object = self
            .client
            .write_object(&self.bucket_name, &object_name, bytes_data)
            .set_content_type(&mime_type)
            .send_buffered()
            .await
            .context("Failed to upload file to Google Cloud Storage")?;

        let url = format!(
            "https://storage.googleapis.com/{}/{}",
            self.bucket_name, object.name
        );

        Ok((url, object.name))
    }

    pub async fn upload_files(
        &self,
        files: Vec<(Vec<u8>, String)>,
        folder: &str,
    ) -> Result<Vec<(String, String)>> {
        let mut results = Vec::new();
        for (file_data, filename) in files {
            let result = self.upload_file(file_data, &filename, folder).await?;
            results.push(result);
        }
        Ok(results)
    }
}

fn sanitize_filename(filename: &str) -> String {
    let stem = Path::new(filename)
        .file_stem()
        .and_then(|s| s.to_str())
        .unwrap_or(filename);
    
    stem.chars()
        .map(|c| {
            if c.is_alphanumeric() || c == '-' || c == '_' {
                c
            } else {
                '_'
            }
        })
        .collect()
}
