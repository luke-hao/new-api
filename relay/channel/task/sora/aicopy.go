package sora

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// AICopy documents JSON video submissions with publicly accessible reference
// URLs. Files from the studio are uploaded once, then represented by URL + role.
// See https://api.aione.help/docs/api/video-plugin-api.md (2026-08-19).
func (a *TaskAdaptor) buildAICopyVideoBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	extra := make(map[string]any, len(req.Metadata))
	for key, value := range req.Metadata {
		extra[key] = value
	}
	// size is a pixel dimension in the public protocol, never "720p".
	// The studio selects resolution and aspect ratio explicitly in extra.
	body := map[string]any{"model": info.UpstreamModelName, "prompt": req.Prompt}
	if req.Seconds != "" {
		body["seconds"] = req.Seconds
	} else if req.Duration > 0 {
		body["seconds"] = req.Duration
	}
	if req.Size != "" && !strings.HasSuffix(strings.ToLower(req.Size), "p") && !strings.EqualFold(req.Size, "2k") && !strings.EqualFold(req.Size, "4k") {
		body["size"] = req.Size
	}
	if _, exists := extra["resolution"]; !exists && (strings.HasSuffix(strings.ToLower(req.Size), "p") || strings.EqualFold(req.Size, "2k") || strings.EqualFold(req.Size, "4k")) {
		extra["resolution"] = req.Size
	}
	// A first/last pair must not also be submitted as a single input_reference.
	_, hasReferences := extra["reference_images"]
	if !hasReferences {
		image := req.InputReference
		if image == "" {
			image = req.Image
		}
		if image == "" && len(req.Images) == 1 {
			image = req.Images[0]
		}
		if image != "" {
			publicURL, err := a.uploadAICopyReference(c, info, image, "image")
			if err != nil {
				return nil, err
			}
			body["input_reference"] = map[string]string{"url": publicURL}
		} else if len(req.Images) > 1 {
			refs := make([]map[string]string, 0, len(req.Images))
			for _, image := range req.Images {
				refs = append(refs, map[string]string{"url": image, "role": "reference_image"})
			}
			extra["reference_images"] = refs
		}
	}
	if _, exists := extra["reference_videos"]; !exists && len(req.Videos) > 0 {
		extra["reference_videos"] = req.Videos
	}
	if _, exists := extra["reference_audios"]; !exists && len(req.Audios) > 0 {
		extra["reference_audios"] = req.Audios
	}
	for field, kind := range map[string]string{"reference_images": "image", "reference_videos": "video", "reference_audios": "audio"} {
		value, exists := extra[field]
		if !exists {
			continue
		}
		encoded, err := common.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("%s is invalid", field)
		}
		var items []any
		if err := common.Unmarshal(encoded, &items); err != nil {
			return nil, fmt.Errorf("%s is invalid", field)
		}
		for i, item := range items {
			reference := ""
			entry := map[string]any{}
			switch v := item.(type) {
			case string:
				reference = v
			case map[string]any:
				for k, v := range v {
					entry[k] = v
				}
				for _, key := range []string{"url", "image_url", "video_url", "audio_url"} {
					if text, ok := v[key].(string); ok && text != "" {
						reference = text
						break
					}
				}
			}
			if reference == "" {
				return nil, fmt.Errorf("%s[%d] requires a URL", field, i)
			}
			publicURL, err := a.uploadAICopyReference(c, info, reference, kind)
			if err != nil {
				return nil, err
			}
			entry["url"] = publicURL
			delete(entry, "image_url")
			delete(entry, "video_url")
			delete(entry, "audio_url")
			items[i] = entry
		}
		extra[field] = items
	}
	if len(extra) > 0 {
		body["extra"] = extra
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	c.Request.Header.Set("Content-Type", "application/json")
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) uploadAICopyReference(c *gin.Context, info *relaycommon.RelayInfo, reference, kind string) (string, error) {
	if !strings.HasPrefix(reference, "data:") {
		parsed, err := url.Parse(reference)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return "", fmt.Errorf("%s reference requires an HTTP(S) URL", kind)
		}
		return reference, nil
	}
	header, encoded, ok := strings.Cut(reference, ",")
	if !ok || !strings.HasSuffix(header, ";base64") {
		return "", fmt.Errorf("invalid %s data URI", kind)
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid %s data URI", kind)
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	extension := map[string]string{"image": ".jpg", "video": ".mp4", "audio": ".wav"}[kind]
	// Preserve the declared MIME type; extensions alone are insufficient for
	// upstream media validation (PNG, WebP, MP3, etc.).
	mimeType := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
	if ext, ok := map[string]string{"image/png": ".png", "image/webp": ".webp", "image/jpeg": ".jpg", "audio/mpeg": ".mp3", "audio/wav": ".wav", "audio/mp4": ".m4a", "video/mp4": ".mp4", "video/webm": ".webm"}[mimeType]; ok {
		extension = ext
	}
	partHeader := make(map[string][]string)
	partHeader["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="reference%s"`, kind, extension)}
	partHeader["Content-Type"] = []string{mimeType}
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return "", err
	}
	if _, err = part.Write(data); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, strings.TrimRight(a.baseURL, "/")+"/v1/uploads", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s reference upload failed", kind)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s reference upload returned HTTP %d", kind, resp.StatusCode)
	}
	var response map[string]any
	if err := common.DecodeJson(io.LimitReader(resp.Body, 1<<20), &response); err != nil {
		return "", fmt.Errorf("%s reference upload returned invalid JSON", kind)
	}
	uploaded := aicopyUploadURL(response)
	if strings.HasPrefix(uploaded, "/") {
		base, err := url.Parse(a.baseURL)
		if err == nil {
			ref, parseErr := url.Parse(uploaded)
			if parseErr == nil {
				uploaded = base.ResolveReference(ref).String()
			}
		}
	}
	parsed, err := url.Parse(uploaded)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("%s reference upload did not return an HTTP(S) URL", kind)
	}
	return uploaded, nil
}

func aicopyUploadURL(response map[string]any) string {
	for _, key := range []string{"url", "file_url", "image_url", "video_url", "audio_url"} {
		if value, ok := response[key].(string); ok && value != "" {
			return value
		}
	}
	for _, key := range []string{"urls", "file_urls", "image_urls"} {
		if values, ok := response[key].([]any); ok && len(values) > 0 {
			if value, ok := values[0].(string); ok {
				return value
			}
		}
	}
	if nested, ok := response["data"].(map[string]any); ok {
		return aicopyUploadURL(nested)
	}
	return ""
}
