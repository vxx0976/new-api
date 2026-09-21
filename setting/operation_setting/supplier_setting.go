package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// SupplierSetting 供应商入驻相关配置
type SupplierSetting struct {
	// ChannelNames maps a channel type to the channel names a supplier may pick
	// for that type. A type without names is closed to suppliers.
	ChannelNames map[int][]string `json:"channel_names"`
}

var supplierSetting = SupplierSetting{
	ChannelNames: map[int][]string{},
}

func init() {
	config.GlobalConfig.Register("supplier_setting", &supplierSetting)
}

func GetSupplierSetting() *SupplierSetting {
	return &supplierSetting
}

// IsSupplierChannelNameAllowed reports whether name is one of the configured
// channel names for channelType.
func IsSupplierChannelNameAllowed(channelType int, name string) bool {
	for _, allowed := range supplierSetting.ChannelNames[channelType] {
		if allowed == name {
			return true
		}
	}
	return false
}
