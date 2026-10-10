package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// Smart routing strategies (P8-2/P8-3).
const (
	SmartRoutingCheapest   = "cheapest"
	SmartRoutingTrustFirst = "trust_first"
	SmartRoutingBalanced   = "balanced"
)

type SmartRoutingSetting struct {
	Enabled  bool   `json:"enabled"`
	Strategy string `json:"strategy"` // cheapest | trust_first | balanced
}

var smartRoutingSetting = SmartRoutingSetting{
	Enabled:  true,
	Strategy: SmartRoutingBalanced,
}

func init() {
	config.GlobalConfig.Register("smart_routing_setting", &smartRoutingSetting)
}

func GetSmartRoutingSetting() *SmartRoutingSetting {
	return &smartRoutingSetting
}

func UpdateSmartRoutingSetting(enabled bool, strategy string) {
	smartRoutingSetting.Enabled = enabled
	switch strategy {
	case SmartRoutingCheapest, SmartRoutingTrustFirst, SmartRoutingBalanced:
		smartRoutingSetting.Strategy = strategy
	default:
		smartRoutingSetting.Strategy = SmartRoutingBalanced
	}
}
