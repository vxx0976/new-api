package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetPricingSupplierTestTables(t *testing.T) {
	t.Helper()
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Model{}, &Vendor{}, &User{}))
	tables := []string{"abilities", "channels", "models", "vendors", "users"}
	clean := func() {
		for _, table := range tables {
			require.NoError(t, DB.Exec("DELETE FROM "+table).Error)
		}
		InitChannelCache()
		InvalidatePricingCache()
	}
	clean()
	t.Cleanup(func() {
		clean()
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
	})
}

// insertPricingSupplier creates an approved supplier user owning channels.
func insertPricingSupplier(t *testing.T, userId int, companyName string) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:          userId,
		Username:    fmt.Sprintf("supplier-%d", userId),
		Password:    "password",
		Role:        common.RoleSupplierUser,
		Status:      common.UserStatusEnabled,
		AffCode:     fmt.Sprintf("aff-%d", userId),
		CompanyName: companyName,
	}).Error)
}

// insertPricingOwnedAbility wires one enabled ability to a channel owned by ownerId (0 = platform).
func insertPricingOwnedAbility(t *testing.T, channelId int, ownerId int, modelName string) {
	t.Helper()
	require.NoError(t, DB.Create(&Channel{
		Id:      channelId,
		Type:    constant.ChannelTypeOpenAI,
		Key:     fmt.Sprintf("key-%d", channelId),
		Status:  common.ChannelStatusEnabled,
		Name:    fmt.Sprintf("channel-%d", channelId),
		OwnerId: ownerId,
	}).Error)
	require.NoError(t, DB.Create(&Ability{
		Group:     "default",
		Model:     modelName,
		ChannelId: channelId,
		Enabled:   true,
	}).Error)
}

func pricingSuppliersByModel(t *testing.T) map[string][]string {
	t.Helper()
	InitChannelCache()
	byModel := make(map[string][]string)
	for _, pricing := range GetPricing() {
		byModel[pricing.ModelName] = pricing.Suppliers
	}
	return byModel
}

func TestPricingSuppliersComeFromChannelOwnership(t *testing.T) {
	resetPricingSupplierTestTables(t)

	insertPricingSupplier(t, 501, "Zeta Cloud")
	insertPricingSupplier(t, 502, "Acme Labs")
	insertPricingSupplier(t, 503, "Acme Labs")
	// supplier without a company name must not surface as an empty supplier entry
	insertPricingSupplier(t, 504, "")

	// served by two suppliers plus a platform channel
	insertPricingOwnedAbility(t, 601, 501, "gpt-4o")
	insertPricingOwnedAbility(t, 602, 502, "gpt-4o")
	insertPricingOwnedAbility(t, 603, 0, "gpt-4o")
	// two supplier users sharing one company name collapse to a single entry
	insertPricingOwnedAbility(t, 604, 503, "shared-company-model")
	insertPricingOwnedAbility(t, 605, 502, "shared-company-model")
	// platform-owned only
	insertPricingOwnedAbility(t, 606, 0, "platform-only-model")
	// owner exists but has no company name
	insertPricingOwnedAbility(t, 607, 504, "nameless-supplier-model")

	byModel := pricingSuppliersByModel(t)

	assert.Equal(t, []string{"Acme Labs", "Zeta Cloud"}, byModel["gpt-4o"])
	assert.Equal(t, []string{"Acme Labs"}, byModel["shared-company-model"])
	assert.Empty(t, byModel["platform-only-model"])
	assert.Empty(t, byModel["nameless-supplier-model"])
}

func TestPricingSuppliersAreStableAcrossRebuilds(t *testing.T) {
	resetPricingSupplierTestTables(t)

	insertPricingSupplier(t, 511, "Zeta Cloud")
	insertPricingSupplier(t, 512, "Acme Labs")
	insertPricingSupplier(t, 513, "Mu Systems")
	insertPricingOwnedAbility(t, 611, 511, "gpt-4o")
	insertPricingOwnedAbility(t, 612, 512, "gpt-4o")
	insertPricingOwnedAbility(t, 613, 513, "gpt-4o")

	first := pricingSuppliersByModel(t)["gpt-4o"]
	require.Equal(t, []string{"Acme Labs", "Mu Systems", "Zeta Cloud"}, first)

	for i := 0; i < 3; i++ {
		assert.Equal(t, first, pricingSuppliersByModel(t)["gpt-4o"])
	}
}

func TestPricingSuppliersDropDisabledAbilities(t *testing.T) {
	resetPricingSupplierTestTables(t)

	insertPricingSupplier(t, 521, "Zeta Cloud")
	insertPricingSupplier(t, 522, "Acme Labs")
	insertPricingOwnedAbility(t, 621, 521, "gpt-4o")
	insertPricingOwnedAbility(t, 622, 522, "gpt-4o")

	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ?", 622).Update("enabled", false).Error)

	assert.Equal(t, []string{"Zeta Cloud"}, pricingSuppliersByModel(t)["gpt-4o"])
}
