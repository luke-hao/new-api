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

	if durationStr := getFormValue("seconds"); durationStr != "" {
		duration, err := strconv.Atoi(durationStr)
		if err != nil {
			return req, fmt.Errorf("seconds is invalid")
		}
		req.Duration = duration
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}
	fileImages, err := readTaskInputReferenceFiles(form)
	if err != nil {
		return req, err
	}
	if len(fileImages) > 0 {
		req.Images = append(req.Images, fileImages...)
		req.Image = req.Images[0]
		req.InputReference = req.Images[0]
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

func readTaskInputReferenceFiles(form *multipart.Form) ([]string, error) {
	files := form.File["input_reference"]
	images := make([]string, 0, len(files))
	for _, fileHeader := range files {
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
		images = append(images, "data:"+contentType+";base64,"+base64.StdEncoding.EncodeToString(data))
	}
	return images, nil
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var prompt string
	var model string
	var seconds int
	var size string
	var hasInputReference bool

	var req TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}

	prompt = req.Prompt
	model = req.Model
	size = req.Size
	seconds, _ = strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" {
		req.Images = []string{req.InputReference}
	}
	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
		fileImages, err := readTaskInputReferenceFiles(form)
		_ = form.RemoveAll()
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
		if len(fileImages) > 0 {
			req.Images = fileImages
			req.Image = fileImages[0]
			req.InputReference = fileImages[0]
		}
	}
	if taskErr := validateTaskQuantityLimits(&req); taskErr != nil {
		return taskErr
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
		"prompt":          true,
		"model":           true,
		"group":           true,
		"mode":            true,
		"image":           true,
		"images":          true,
		"size":            true,
		"duration":        true,
		"seconds":         true,
		"metadata":        true,
		"input_reference": true, // Sora 特有字段
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
	}
	// 为了metadata字段的兼容性，统一UnmarshalBodyReusable
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}
	if taskErr := validateTaskQuantityLimits(&req); taskErr != nil {
		return taskErr
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
