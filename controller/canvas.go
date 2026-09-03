package controller

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	canvasSSODefaultIssuer   = "new-api"
	canvasSSODefaultAudience = "infinite-canvas"
	canvasSSOTicketTTL       = 5 * time.Minute
)

type canvasSSOClaims struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	IsRoot      bool   `json:"is_root"`
	jwt.RegisteredClaims
}

type canvasModelGroup struct {
	Group       string   `json:"group"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Ratio       float64  `json:"ratio"`
	Models      []string `json:"models"`
}

func canvasGroupType(group string) string {
	switch {
	case strings.HasPrefix(group, "生图分组-"):
		return "image"
	case group == "视频生成":
		return "video"
	default:
		return "text"
	}
}

func createCanvasSSOTicket(user *model.User, now time.Time, secret string) (string, time.Time, error) {
	if len(secret) < 32 {
		return "", time.Time{}, errors.New("CANVAS_SSO_SECRET must contain at least 32 characters")
	}

	expiresAt := now.Add(canvasSSOTicketTTL)
	claims := canvasSSOClaims{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		IsRoot:      user.Role == common.RoleRootUser,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    common.GetEnvOrDefaultString("CANVAS_SSO_ISSUER", canvasSSODefaultIssuer),
			Subject:   strconv.Itoa(user.Id),
			Audience:  jwt.ClaimStrings{common.GetEnvOrDefaultString("CANVAS_SSO_AUDIENCE", canvasSSODefaultAudience)},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}

	ticket, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return ticket, expiresAt, err
}

func CreateCanvasSSOTicket(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ticket, expiresAt, err := createCanvasSSOTicket(
		user,
		time.Now().UTC(),
		common.GetEnvOrDefaultString("CANVAS_SSO_SECRET", ""),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"ticket":     ticket,
		"expires_at": expiresAt.Unix(),
	})
}

func GetCanvasCapabilities(c *gin.Context) {
	user, err := model.GetUserCache(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	usableGroups := service.GetUserUsableGroups(user.Group)
	queryGroups := make([]string, 0, len(usableGroups))
	for group := range usableGroups {
		if group != "" && group != "auto" {
			queryGroups = append(queryGroups, group)
		}
	}
	sort.Strings(queryGroups)

	abilities, err := model.GetGroupsEnabledAbilitiesWithChannels(queryGroups)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	modelsByGroup := make(map[string]map[string]struct{}, len(queryGroups))
	for _, ability := range abilities {
		if _, ok := usableGroups[ability.Group]; !ok {
			continue
		}
		if modelsByGroup[ability.Group] == nil {
			modelsByGroup[ability.Group] = make(map[string]struct{})
		}
		modelsByGroup[ability.Group][ability.Model] = struct{}{}
	}

	groups := make([]canvasModelGroup, 0, len(modelsByGroup))
	for _, group := range queryGroups {
		modelSet := modelsByGroup[group]
		if len(modelSet) == 0 {
			continue
		}
		models := make([]string, 0, len(modelSet))
		for modelName := range modelSet {
			models = append(models, modelName)
		}
		sort.Strings(models)
		groups = append(groups, canvasModelGroup{
			Group:       group,
			Description: usableGroups[group],
			Type:        canvasGroupType(group),
			Ratio:       service.GetUserGroupRatio(user.Group, group),
			Models:      models,
		})
	}

	typeRank := map[string]int{"text": 0, "image": 1, "video": 2}
	sort.Slice(groups, func(i, j int) bool {
		if typeRank[groups[i].Type] != typeRank[groups[j].Type] {
			return typeRank[groups[i].Type] < typeRank[groups[j].Type]
		}
		return groups[i].Group < groups[j].Group
	})

	videoAbilities := make([]model.AbilityWithChannel, 0)
	for _, ability := range abilities {
		if ability.Group == "视频生成" {
			videoAbilities = append(videoAbilities, ability)
		}
	}
	videoCapabilities := make([]playgroundVideoGroupCapability, 0, 1)
	if _, visible := usableGroups["视频生成"]; visible {
		if videoModels := collectPlaygroundVideoModels(videoAbilities); len(videoModels) > 0 {
			videoCapabilities = append(videoCapabilities, playgroundVideoGroupCapability{
				Group:  "视频生成",
				Desc:   usableGroups["视频生成"],
				Ratio:  service.GetUserGroupRatio(user.Group, "视频生成"),
				Models: videoModels,
			})
		}
	}

	common.ApiSuccess(c, gin.H{"groups": groups, "video_capabilities": videoCapabilities})
}
