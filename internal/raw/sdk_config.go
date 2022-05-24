package raw

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/lidofinance/cosmos-query-relayer/internal/config"
)

// SetSDKConfig sets GLOBAL values for prefixes for cosmos-sdk when parsing addresses and so on
// Apparently, there is no way around that for now
// Without this some functions just does not work as intended
func SetSDKConfig(cfg config.LidoChainConfig) {
	sdkCfg := sdk.GetConfig()
	sdkCfg.SetBech32PrefixForAccount(cfg.ChainPrefix, cfg.ChainPrefix+sdk.PrefixPublic)
	sdkCfg.Seal()
}
