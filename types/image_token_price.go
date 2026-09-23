package types

// ImageTokenPrice contains USD prices per million tokens, before group ratios.
type ImageTokenPrice struct {
	Input            float64 `json:"input"`
	Output           float64 `json:"output"`
	ImageInput       float64 `json:"image_input"`
	ImageOutput      float64 `json:"image_output"`
	CachedInput      float64 `json:"cached_input"`
	CachedImageInput float64 `json:"cached_image_input"`
	CacheCreation    float64 `json:"cache_creation"`
}
