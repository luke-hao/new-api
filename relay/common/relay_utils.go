package common

import (
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

const (
	maxPlaygroundImageBytes = int64(15 << 20)
	maxPlaygroundVideoBytes = int64(160 << 20)
	maxPlaygroundAudioBytes = int64(50 << 20)
)

type HasPrompt interface {
	GetPrompt() string
}

type HasImage interface {
	HasImage() bool
}

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case constant.ChannelTypeOpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case constant.ChannelTypeAzure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}

func GetAPIVersion(c *gin.Context) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion == "" {
		apiVersion = c.GetString("api_version")
	}
	return apiVersion
}

func createTaskError(err error, code string, statusCode int, localError bool) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: localError,
		Error:      err,
	}
}

func storeTaskRequest(c *gin.Context, info *RelayInfo, action string, requestObj TaskSubmitReq) {
	info.Action = action
	c.Set("task_request", requestObj)
}
func GetTaskRequest(c *gin.Context) (TaskSubmitReq, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return TaskSubmitReq{}, fmt.Errorf("request not found in context")
	}
	req, ok := v.(TaskSubmitReq)
	if !ok {
		return TaskSubmitReq{}, fmt.Errorf("invalid task request type")
	}
	return req, nil
}

func validatePrompt(prompt string) *dto.TaskError {
	if strings.TrimSpace(prompt) == "" {
		return createTaskError(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

func validateMultipartTaskRequest(c *gin.Context, info *RelayInfo, action string) (TaskSubmitReq, error) {
	var req TaskSubmitReq
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return req, err
	}
	defer form.RemoveAll()

	formData := form.Value
	getFormValue := func(key string) string {
		if values := formData[key]; len(values) > 0 {
			return values[0]
		}
		return ""
	}
	req = TaskSubmitReq{
		Prompt:   getFormValue("prompt"),
		Model:    getFormValue("model"),
		Mode:     getFormValue("mode"),
		Image:    getFormValue("image"),
		Size:     getFormValue("size"),
		Metadata: make(map[string]interface{}),
	}
	for _, field := range []string{"metadata", "extra"} {
		raw := strings.TrimSpace(getFormValue(field))
		if raw == "" {
			continue
		}
		values := make(map[string]interface{})
		if err := common.Unmarshal([]byte(raw), &values); err != nil {
			return req, fmt.Errorf("%s is invalid", field)
		}
		for key, value := range values {
			req.Metadata[key] = value
		}
	}

	if durationStr := getFormValue("seconds"); durationStr != "" {
		duration, err := strconv.Atoi(durationStr)
		if err != nil {
			return req, fmt.Errorf("seconds is invalid")
		}
		req.Duration = duration
	} else if durationStr := getFormValue("duration"); durationStr != "" {
		duration, err := strconv.Atoi(durationStr)
		if err != nil {
			return req, fmt.Errorf("duration is invalid")
		}
		req.Duration = duration
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}
	fileImages, err := readTaskReferenceFiles(form, "input_reference", "image/", maxPlaygroundImageBytes)
	if err != nil {
		return req, err
	}
	if len(fileImages) > 0 {
		req.Images = append(req.Images, fileImages...)
		req.Image = req.Images[0]
		req.InputReference = req.Images[0]
	}
	referenceImages, err := readTaskReferenceFiles(form, "reference_images", "image/", maxPlaygroundImageBytes)
	if err != nil {
		return req, err
	}
	if len(referenceImages) > 0 {
		req.Images = append(req.Images, referenceImages...)
		req.Metadata["reference_images"] = taskReferenceMetadata(referenceImages, "reference_image")
	}
	referenceVideos, err := readTaskReferenceFiles(form, "reference_videos", "video/", maxPlaygroundVideoBytes)
	if err != nil {
		return req, err
	}
	if len(referenceVideos) > 0 {
		req.Videos = append(req.Videos, referenceVideos...)
		req.Metadata["reference_videos"] = taskReferenceMetadata(referenceVideos, "")
	}
	referenceAudios, err := readTaskReferenceFiles(form, "reference_audios", "audio/", maxPlaygroundAudioBytes)
	if err != nil {
		return req, err
	}
	if len(referenceAudios) > 0 {
		req.Audios = append(req.Audios, referenceAudios...)
		req.Metadata["reference_audios"] = taskReferenceMetadata(referenceAudios, "")
	}
	if normalizeVideoMode(req.Mode) == "first_last" && len(req.Images) >= 2 {
		req.Metadata["reference_images"] = []map[string]string{
			{"url": req.Images[0], "role": "first_frame"},
			{"url": req.Images[1], "role": "last_frame"},
		}
	}

	for key, values := range formData {
		if len(values) > 0 && !isKnownTaskField(key) {
			if intVal, err := strconv.Atoi(values[0]); err == nil {
				req.Metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(values[0], 64); err == nil {
				req.Metadata[key] = floatVal
			} else {
				req.Metadata[key] = values[0]
			}
		}
	}
	return req, nil
}

func taskReferenceMetadata(references []string, role string) []map[string]string {
	metadata := make([]map[string]string, 0, len(references))
	for _, reference := range references {
		item := map[string]string{"url": reference}
		if role != "" {
			item["role"] = role
		}
		metadata = append(metadata, item)
	}
	return metadata
}

func readTaskReferenceFiles(form *multipart.Form, field, contentPrefix string, maxBytes int64) ([]string, error) {
	files := form.File[field]
	references := make([]string, 0, len(files))
	for _, fileHeader := range files {
		if fileHeader.Size > maxBytes {
			return nil, fmt.Errorf("%s file %s exceeds size limit", field, fileHeader.Filename)
		}
		file, err := fileHeader.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			return nil, readErr
		}
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = http.DetectContentType(data)
		}
		if !strings.HasPrefix(strings.ToLower(contentType), contentPrefix) {
			return nil, fmt.Errorf("%s file %s has invalid content type", field, fileHeader.Filename)
		}
		references = append(references, "data:"+contentType+";base64,"+base64.StdEncoding.EncodeToString(data))
	}
	return references, nil
}

func readTaskInputReferenceFiles(form *multipart.Form) ([]string, error) {
	return readTaskReferenceFiles(form, "input_reference", "image/", maxPlaygroundImageBytes)
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var prompt string
	var model string
	var seconds int
	var size string
	var hasInputReference bool

	var req TaskSubmitReq
	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		parsed, err := validateMultipartTaskRequest(c, info, constant.TaskActionGenerate)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
		req = parsed
	} else if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}

	prompt = req.Prompt
	model = req.Model
	size = req.Size
	seconds, _ = strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" && len(req.Images) == 0 {
		req.Images = []string{req.InputReference}
	}
	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		req.Images = []string{req.Image}
	}
	if taskErr := validateTaskQuantityLimits(&req); taskErr != nil {
		return taskErr
	}
	if strings.HasPrefix(c.Request.URL.Path, "/pg/videos") {
		if taskErr := validatePlaygroundVideoMedia(&req); taskErr != nil {
			return taskErr
		}
		if taskErr := validatePlaygroundVideoParameters(c, info, &req); taskErr != nil {
			return taskErr
		}
	}

	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	if req.HasImage() {
		hasInputReference = true
	}

	if taskErr := validatePrompt(prompt); taskErr != nil {
		return taskErr
	}

	action := constant.TaskActionTextGenerate
	if hasInputReference {
		action = constant.TaskActionGenerate
	}
	if strings.HasPrefix(model, "sora-2") {

		if size == "" {
			size = "720x1280"
		}

		if seconds <= 0 {
			seconds = 4
		}

		if model == "sora-2" && !lo.Contains([]string{"720x1280", "1280x720"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		if model == "sora-2-pro" && !lo.Contains([]string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		// OtherRatios 已移到 Sora adaptor 的 EstimateBilling 中设置
	}

	storeTaskRequest(c, info, action, req)

	return nil
}

func isKnownTaskField(field string) bool {
	knownFields := map[string]bool{
		"prompt":           true,
		"model":            true,
		"group":            true,
		"mode":             true,
		"image":            true,
		"images":           true,
		"size":             true,
		"duration":         true,
		"seconds":          true,
		"metadata":         true,
		"extra":            true,
		"input_reference":  true, // Sora 特有字段
		"reference_images": true,
		"reference_videos": true,
		"reference_audios": true,
	}
	return knownFields[field]
}

func ValidateBasicTaskRequest(c *gin.Context, info *RelayInfo, action string) *dto.TaskError {
	var err error
	contentType := c.GetHeader("Content-Type")
	var req TaskSubmitReq
	if strings.HasPrefix(contentType, "multipart/form-data") {
		req, err = validateMultipartTaskRequest(c, info, action)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
	} else if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}
	if taskErr := validateTaskQuantityLimits(&req); taskErr != nil {
		return taskErr
	}
	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容旧的 image 字段。
		req.Images = []string{req.Image}
	}
	if req.InputReference != "" && len(req.Images) == 0 {
		req.Images = []string{req.InputReference}
	}

	if strings.HasPrefix(c.Request.URL.Path, "/pg/videos") {
		if taskErr := validatePlaygroundVideoMedia(&req); taskErr != nil {
			return taskErr
		}
		if taskErr := validatePlaygroundVideoParameters(c, info, &req); taskErr != nil {
			return taskErr
		}
	}

	if taskErr := validatePrompt(req.Prompt); taskErr != nil {
		return taskErr
	}

	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{req.Image}
	}

	storeTaskRequest(c, info, action, req)
	return nil
}

func validateTaskQuantityLimits(req *TaskSubmitReq) *dto.TaskError {
	if req == nil {
		return nil
	}
	if taskErr := validateTaskDurationValue("duration", req.Duration); taskErr != nil {
		return taskErr
	}
	if strings.TrimSpace(req.Seconds) != "" {
		seconds, err := strconv.Atoi(strings.TrimSpace(req.Seconds))
		if err != nil {
			return createTaskError(fmt.Errorf("seconds is invalid"), "invalid_request", http.StatusBadRequest, true)
		}
		if taskErr := validateTaskDurationValue("seconds", seconds); taskErr != nil {
			return taskErr
		}
	}
	for _, key := range []string{"duration", "durationSeconds", "duration_seconds"} {
		if taskErr := validateTaskMetadataDuration(req.Metadata, key); taskErr != nil {
			return taskErr
		}
	}
	return nil
}

func normalizeVideoMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "image", "first_frame":
		return "first_frame"
	case "first_last":
		return "first_last"
	case "reference", "multi_reference":
		return "reference"
	case "video_edit":
		return "video_edit"
	case "text":
		return "text"
	default:
		return ""
	}
}

func validatePlaygroundVideoMedia(req *TaskSubmitReq) *dto.TaskError {
	if req == nil {
		return nil
	}
	if len(req.Images) > 30 {
		return createTaskError(fmt.Errorf("reference_images supports at most 30 items"), "invalid_request", http.StatusBadRequest, true)
	}
	if len(req.Videos) > 10 {
		return createTaskError(fmt.Errorf("reference_videos supports at most 10 items"), "invalid_request", http.StatusBadRequest, true)
	}
	if len(req.Audios) > 10 {
		return createTaskError(fmt.Errorf("reference_audios supports at most 10 items"), "invalid_request", http.StatusBadRequest, true)
	}

	mode := normalizeVideoMode(req.Mode)
	if strings.TrimSpace(req.Mode) != "" && mode == "" {
		return createTaskError(fmt.Errorf("mode is invalid"), "invalid_request", http.StatusBadRequest, true)
	}
	switch mode {
	case "text":
		if len(req.Images)+len(req.Videos)+len(req.Audios) > 0 {
			return createTaskError(fmt.Errorf("text mode does not accept reference media"), "invalid_request", http.StatusBadRequest, true)
		}
	case "first_frame":
		if len(req.Images) != 1 || len(req.Videos)+len(req.Audios) > 0 {
			return createTaskError(fmt.Errorf("first_frame mode requires exactly one image"), "invalid_request", http.StatusBadRequest, true)
		}
	case "first_last":
		if len(req.Images) != 2 || len(req.Videos)+len(req.Audios) > 0 {
			return createTaskError(fmt.Errorf("first_last mode requires exactly two images"), "invalid_request", http.StatusBadRequest, true)
		}
	case "reference":
		if len(req.Images)+len(req.Videos)+len(req.Audios) == 0 {
			return createTaskError(fmt.Errorf("reference mode requires at least one media item"), "invalid_request", http.StatusBadRequest, true)
		}
	case "video_edit":
		if len(req.Videos) == 0 || len(req.Images)+len(req.Audios) > 0 {
			return createTaskError(fmt.Errorf("video_edit mode requires a reference video"), "invalid_request", http.StatusBadRequest, true)
		}
	}
	return nil
}

func resolvePlaygroundValidationModel(c *gin.Context, modelName string) string {
	rawMapping := strings.TrimSpace(c.GetString("model_mapping"))
	if rawMapping == "" || rawMapping == "{}" {
		return modelName
	}
	mapping := make(map[string]string)
	if err := common.Unmarshal([]byte(rawMapping), &mapping); err != nil {
		return modelName
	}
	current := modelName
	visited := map[string]struct{}{current: {}}
	for {
		next := strings.TrimSpace(mapping[current])
		if next == "" || next == current {
			return current
		}
		if _, exists := visited[next]; exists {
			return modelName
		}
		visited[next] = struct{}{}
		current = next
	}
}

func taskMetadataString(metadata map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := metadata[key]; ok {
			if text, ok := value.(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func validatePlaygroundVideoParameters(c *gin.Context, info *RelayInfo, req *TaskSubmitReq) *dto.TaskError {
	if req == nil {
		return nil
	}
	modelName := resolvePlaygroundValidationModel(c, req.Model)
	channelType := c.GetInt("channel_type")
	if info != nil && info.ChannelType != 0 {
		channelType = info.ChannelType
	}
	profile, ok := common.GetPlaygroundVideoCapability(channelType, modelName)
	if !ok {
		return nil
	}
	mode := normalizeVideoMode(req.Mode)
	if mode != "" && !lo.Contains(profile.Modes, mode) {
		return createTaskError(fmt.Errorf("mode is not supported by model %s", req.Model), "invalid_request", http.StatusBadRequest, true)
	}
	duration := req.Duration
	if duration == 0 && strings.TrimSpace(req.Seconds) != "" {
		duration, _ = strconv.Atoi(strings.TrimSpace(req.Seconds))
	}
	if duration > 0 && !lo.Contains(profile.Durations, duration) {
		return createTaskError(fmt.Errorf("duration is not supported by model %s", req.Model), "invalid_request", http.StatusBadRequest, true)
	}
	ratio := taskMetadataString(req.Metadata, "aspect_ratio", "aspectRatio", "ratio")
	if ratio != "" && len(profile.AspectRatios) > 0 && !lo.Contains(profile.AspectRatios, ratio) {
		return createTaskError(fmt.Errorf("aspect_ratio is not supported by model %s", req.Model), "invalid_request", http.StatusBadRequest, true)
	}
	resolution := strings.ToLower(taskMetadataString(req.Metadata, "resolution"))
	if resolution != "" && len(profile.Resolutions) > 0 && !lo.Contains(profile.Resolutions, resolution) {
		return createTaskError(fmt.Errorf("resolution is not supported by model %s", req.Model), "invalid_request", http.StatusBadRequest, true)
	}
	if len(req.Images) > profile.MaxImageReferences || len(req.Videos) > profile.MaxVideoReferences || len(req.Audios) > profile.MaxAudioReferences {
		return createTaskError(fmt.Errorf("reference media exceeds the limits for model %s", req.Model), "invalid_request", http.StatusBadRequest, true)
	}
	if taskErr := validatePlaygroundVideoReferenceBytes(req, mode, profile); taskErr != nil {
		return taskErr
	}
	return nil
}

func validatePlaygroundVideoReferenceBytes(req *TaskSubmitReq, mode string, profile common.PlaygroundVideoCapability) *dto.TaskError {
	imageLimit := profile.MaxImageBytes
	videoLimit := profile.MaxVideoBytes
	if mode == "video_edit" {
		videoLimit = profile.MaxVideoEditBytes
	}
	for _, reference := range req.Images {
		if err := validatePlaygroundDataReference(reference, "image/", imageLimit); err != nil {
			return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
		}
	}
	for _, reference := range req.Videos {
		if err := validatePlaygroundDataReference(reference, "video/", videoLimit); err != nil {
			return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
		}
	}
	for _, reference := range req.Audios {
		if err := validatePlaygroundDataReference(reference, "audio/", profile.MaxAudioBytes); err != nil {
			return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
		}
	}
	return nil
}

func validatePlaygroundDataReference(reference, contentPrefix string, maxBytes int64) error {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(reference)), "data:") {
		return nil
	}
	headerEnd := strings.IndexByte(reference, ',')
	if headerEnd < 0 {
		return fmt.Errorf("reference media is invalid")
	}
	header := strings.ToLower(reference[:headerEnd])
	if !strings.Contains(header, contentPrefix) {
		return fmt.Errorf("reference media has invalid content type")
	}
	encoded := strings.TrimSpace(reference[headerEnd+1:])
	if strings.Contains(header, ";base64") {
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("reference media is invalid")
		}
		if maxBytes > 0 && int64(len(data)) > maxBytes {
			return fmt.Errorf("reference media exceeds size limit")
		}
		return nil
	}
	if maxBytes > 0 && int64(len(encoded)) > maxBytes {
		return fmt.Errorf("reference media exceeds size limit")
	}
	return nil
}

func validateTaskDurationValue(field string, value int) *dto.TaskError {
	if value == 0 {
		return nil
	}
	if err := common.ValidateIntRange(field, value, 1, common.MaxVideoDurationSeconds); err != nil {
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

func validateTaskMetadataDuration(metadata map[string]interface{}, key string) *dto.TaskError {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case int:
		return validateTaskDurationValue("metadata."+key, v)
	case int64:
		if v > int64(common.MaxSafeInt()) || v < int64(-common.MaxSafeInt()) {
			return createTaskError(fmt.Errorf("metadata.%s exceeds integer range", key), "invalid_request", http.StatusBadRequest, true)
		}
		return validateTaskDurationValue("metadata."+key, int(v))
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) {
			return createTaskError(fmt.Errorf("metadata.%s is invalid", key), "invalid_request", http.StatusBadRequest, true)
		}
		seconds, err := common.SafeNonNegativeFloatToInt("metadata."+key, v)
		if err != nil {
			return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
		}
		return validateTaskDurationValue("metadata."+key, seconds)
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		seconds, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return createTaskError(fmt.Errorf("metadata.%s is invalid", key), "invalid_request", http.StatusBadRequest, true)
		}
		return validateTaskDurationValue("metadata."+key, seconds)
	default:
		return createTaskError(fmt.Errorf("metadata.%s is invalid", key), "invalid_request", http.StatusBadRequest, true)
	}
}
