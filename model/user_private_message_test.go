package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetPrivateMessageTestState(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
	require.NoError(t, DB.Exec("DELETE FROM options").Error)

	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		GlobalPrivateMessageOptionKey: "",
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		_ = DB.Exec("DELETE FROM users").Error
		_ = DB.Exec("DELETE FROM options").Error
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})
}

func insertPrivateMessageUser(t *testing.T, user *User, setting dto.UserSetting) {
	t.Helper()
	if user.Password == "" {
		user.Password = "hashed-password"
	}
	if user.AffCode == "" {
		user.AffCode = "aff-" + user.Username
	}
	user.SetSetting(setting)
	require.NoError(t, DB.Create(user).Error)
}

func getStoredPrivateMessage(t *testing.T, userId int) *dto.PrivateMessage {
	t.Helper()
	var user User
	require.NoError(t, DB.Unscoped().First(&user, userId).Error)
	return user.GetSetting().PrivateMessage
}

func TestResolveEffectivePrivateMessage(t *testing.T) {
	personal := &dto.PrivateMessage{Id: "personal", Content: "personal", CreatedAt: 20}
	global := &dto.PrivateMessage{Id: "global", Content: "global", CreatedAt: 10}

	assert.Same(t, personal, ResolveEffectivePrivateMessage(personal, nil))
	assert.Same(t, global, ResolveEffectivePrivateMessage(nil, global))
	assert.Same(t, personal, ResolveEffectivePrivateMessage(personal, global))

	newerGlobal := &dto.PrivateMessage{Id: "global-new", Content: "global", CreatedAt: 30}
	assert.Same(t, newerGlobal, ResolveEffectivePrivateMessage(personal, newerGlobal))
}

func TestBroadcastPrivateMessagePersistsGlobalAndExistingUsers(t *testing.T) {
	resetPrivateMessageTestState(t)

	insertPrivateMessageUser(t, &User{Id: 1, Username: "user-a"}, dto.UserSetting{})
	insertPrivateMessageUser(t, &User{Id: 2, Username: "user-b"}, dto.UserSetting{
		PrivateMessage: &dto.PrivateMessage{Id: "old", Content: "old", CreatedAt: 1},
	})
	insertPrivateMessageUser(t, &User{Id: 3, Username: "deleted-user"}, dto.UserSetting{})
	require.NoError(t, DB.Delete(&User{}, 3).Error)

	message := &dto.PrivateMessage{Id: "global-1", Title: "Title", Content: "hello", CreatedAt: 100}
	count, err := BroadcastPrivateMessage(message)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	globalMessage, err := GetGlobalPrivateMessage()
	require.NoError(t, err)
	require.NotNil(t, globalMessage)
	assert.Equal(t, message.Id, globalMessage.Id)
	assert.Equal(t, message.Content, globalMessage.Content)

	assert.Equal(t, message.Id, getStoredPrivateMessage(t, 1).Id)
	assert.Equal(t, message.Id, getStoredPrivateMessage(t, 2).Id)
	assert.Nil(t, getStoredPrivateMessage(t, 3))
}

func TestBroadcastPrivateMessageWithoutExistingUsersStillReachesNewUsers(t *testing.T) {
	resetPrivateMessageTestState(t)

	message := &dto.PrivateMessage{Id: "global-new-user", Content: "for future users", CreatedAt: 100}
	count, err := BroadcastPrivateMessage(message)
	require.NoError(t, err)
	assert.Zero(t, count)

	effective := GetEffectivePrivateMessage(dto.UserSetting{})
	require.NotNil(t, effective)
	assert.Equal(t, message.Id, effective.Id)
	assert.Equal(t, message.Content, effective.Content)
}

func TestClearGlobalPrivateMessageOnlyClearsMatchingUserMessages(t *testing.T) {
	resetPrivateMessageTestState(t)

	insertPrivateMessageUser(t, &User{Id: 1, Username: "matching-user"}, dto.UserSetting{})
	insertPrivateMessageUser(t, &User{Id: 2, Username: "personal-user"}, dto.UserSetting{})

	global := &dto.PrivateMessage{Id: "global-clear", Content: "global", CreatedAt: 100}
	count, err := BroadcastPrivateMessage(global)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	var personalUser User
	require.NoError(t, DB.First(&personalUser, 2).Error)
	personal := &dto.PrivateMessage{Id: "personal", Content: "personal", CreatedAt: 200}
	setting := personalUser.GetSetting()
	setting.PrivateMessage = personal
	personalUser.SetSetting(setting)
	require.NoError(t, DB.Model(&User{}).Where("id = ?", personalUser.Id).Update("setting", personalUser.Setting).Error)

	clearedCount, err := ClearGlobalPrivateMessage()
	require.NoError(t, err)
	assert.EqualValues(t, 1, clearedCount)

	globalMessage, err := GetGlobalPrivateMessage()
	require.NoError(t, err)
	assert.Nil(t, globalMessage)
	assert.Nil(t, getStoredPrivateMessage(t, 1))
	require.NotNil(t, getStoredPrivateMessage(t, 2))
	assert.Equal(t, personal.Id, getStoredPrivateMessage(t, 2).Id)
}
