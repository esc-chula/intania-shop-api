package models

import "time"

// Promo is a promotional content record.
type Promo struct {
	PromoID   int64     `json:"promo_id"`
	ImgURL    string    `json:"img_url"`
	LinkURL   *string   `json:"link_url"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Banner is a banner content record.
type Banner struct {
	BannerID  int64     `json:"banner_id"`
	ImgURL    string    `json:"img_url"`
	LinkURL   *string   `json:"link_url"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// ContentInput is the approved create/update payload for a promo or banner.
type ContentInput struct {
	ImgURL   *string `json:"img_url"`
	LinkURL  *string `json:"link_url"`
	IsActive *bool   `json:"is_active"`
}
