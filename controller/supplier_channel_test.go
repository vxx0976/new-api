package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const testSupplierId = 42

func setupSupplierChannelTest(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))

	setting := operation_setting.GetSupplierSetting()
	previousNames := setting.ChannelNames
	setting.ChannelNames = map[int][]string{
		constant.ChannelTypeOpenAI: {"openai official key relay"},
	}
	t.Cleanup(func() { setting.ChannelNames = previousNames })
	return db
}

// performSupplierChannelRequest calls a channel handler the way the supplier
// route group does: authenticated as a supplier and scoped to its own channels.
func performSupplierChannelRequest(t *testing.T, handler gin.HandlerFunc, method string, target string, body string) (success bool, data []byte) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", testSupplierId)
	c.Set("role", common.RoleSupplierUser)
	common.SetContextKey(c, constant.ContextKeyChannelOwnerScope, testSupplierId)
	handler(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response.Success, recorder.Body.Bytes()
}

func TestSupplierAddChannelOwnsChannelAndIgnoresClientOwner(t *testing.T) {
	db := setupSupplierChannelTest(t)

	success, _ := performSupplierChannelRequest(t, AddChannel, http.MethodPost, "/api/supplier/channel/",
		`{"mode":"single","channel":{"type":1,"name":"openai official key relay","key":"sk-test","models":"gpt-4o","group":"default","owner_id":7}}`)
	require.True(t, success)

	var channel model.Channel
	require.NoError(t, db.First(&channel).Error)
	assert.Equal(t, testSupplierId, channel.OwnerId)
}

func TestSupplierAddChannelRejectsUnconfiguredName(t *testing.T) {
	db := setupSupplierChannelTest(t)

	cases := map[string]string{
		"custom name":                `{"mode":"single","channel":{"type":1,"name":"my own name","key":"sk-test","models":"gpt-4o","group":"default"}}`,
		"name of another type":       `{"mode":"single","channel":{"type":14,"name":"openai official key relay","key":"sk-test","models":"claude","group":"default"}}`,
		"type without any name list": `{"mode":"single","channel":{"type":24,"name":"","key":"sk-test","models":"gemini","group":"default"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			success, _ := performSupplierChannelRequest(t, AddChannel, http.MethodPost, "/api/supplier/channel/", body)
			assert.False(t, success)
		})
	}

	var count int64
	require.NoError(t, db.Model(&model.Channel{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestSupplierListsOnlyOwnedChannels(t *testing.T) {
	db := setupSupplierChannelTest(t)
	require.NoError(t, db.Create(&[]model.Channel{
		{Name: "mine", Key: "k1", Type: 1, Group: "default", OwnerId: testSupplierId},
		{Name: "platform", Key: "k2", Type: 1, Group: "default"},
		{Name: "other supplier", Key: "k3", Type: 1, Group: "default", OwnerId: 43},
	}).Error)

	success, body := performSupplierChannelRequest(t, GetAllChannels, http.MethodGet, "/api/supplier/channel/?p=1&page_size=20", "")
	require.True(t, success)
	var response struct {
		Data struct {
			Items []model.Channel `json:"items"`
			Total int             `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(body, &response))
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, "mine", response.Data.Items[0].Name)
	assert.Equal(t, 1, response.Data.Total)
}

func TestSupplierCannotUpdateForeignChannel(t *testing.T) {
	db := setupSupplierChannelTest(t)
	foreign := model.Channel{Name: "openai official key relay", Key: "k2", Type: 1, Group: "default", Models: "gpt-4o"}
	require.NoError(t, db.Create(&foreign).Error)

	success, _ := performSupplierChannelRequest(t, UpdateChannel, http.MethodPut, "/api/supplier/channel/",
		`{"id":1,"models":"gpt-4o,gpt-4o-mini","key":"stolen"}`)
	assert.False(t, success)

	var stored model.Channel
	require.NoError(t, db.First(&stored, foreign.Id).Error)
	assert.Equal(t, "gpt-4o", stored.Models)
	assert.Equal(t, "k2", stored.Key)
}

func TestSupplierUpdatesOwnChannelKeyButNotToUnconfiguredName(t *testing.T) {
	db := setupSupplierChannelTest(t)
	own := model.Channel{Name: "openai official key relay", Key: "old", Type: 1, Group: "default", Models: "gpt-4o", OwnerId: testSupplierId}
	require.NoError(t, db.Create(&own).Error)

	success, _ := performSupplierChannelRequest(t, UpdateChannel, http.MethodPut, "/api/supplier/channel/",
		`{"id":1,"key":"new","owner_id":0}`)
	require.True(t, success)

	success, _ = performSupplierChannelRequest(t, UpdateChannel, http.MethodPut, "/api/supplier/channel/",
		`{"id":1,"name":"renamed freely"}`)
	assert.False(t, success)

	var stored model.Channel
	require.NoError(t, db.First(&stored, own.Id).Error)
	assert.Equal(t, "new", stored.Key)
	assert.Equal(t, "openai official key relay", stored.Name)
	assert.Equal(t, testSupplierId, stored.OwnerId)
}

func TestManageUserSupplierApprovalLifecycle(t *testing.T) {
	db := setupSupplierChannelTest(t)
	previousMaster := common.IsMasterNode
	common.IsMasterNode = false
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	require.NoError(t, authz.Init(db))
	applicant := model.User{Id: 42, Username: "acme", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, CompanyName: "Acme", SupplierStatus: common.SupplierStatusPending, AffCode: "acme"}
	plain := model.User{Id: 50, Username: "plain", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "plain"}
	require.NoError(t, db.Create(&applicant).Error)
	require.NoError(t, db.Create(&plain).Error)

	response := performManageUserRequest(t, `{"id":50,"action":"approve_supplier"}`)
	assert.Contains(t, response.Body.String(), `"success":false`)

	response = performManageUserRequest(t, `{"id":42,"action":"approve_supplier"}`)
	require.Contains(t, response.Body.String(), `"success":true`)
	var approved model.User
	require.NoError(t, db.First(&approved, 42).Error)
	assert.Equal(t, common.RoleSupplierUser, approved.Role)
	assert.Equal(t, common.SupplierStatusApproved, approved.SupplierStatus)

	require.NoError(t, db.Create(&model.Channel{Name: "openai official key relay", Key: "k", Type: 1, Group: "default", Models: "gpt-4o", Status: common.ChannelStatusEnabled, OwnerId: 42}).Error)
	response = performManageUserRequest(t, `{"id":42,"action":"demote"}`)
	require.Contains(t, response.Body.String(), `"success":true`)

	var demoted model.User
	require.NoError(t, db.First(&demoted, 42).Error)
	assert.Equal(t, common.RoleCommonUser, demoted.Role)
	var channel model.Channel
	require.NoError(t, db.First(&channel).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, channel.Status)
}
