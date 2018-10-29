package ibc

import (
	"github.com/cosmos/cosmos-sdk/codec"
)

// Register concrete types on codec codec
func RegisterCodec(cdc *codec.Codec) {
	cdc.RegisterConcrete(IBCTransferMsg{}, "cosmos-sdk/IBCTransferMsg/1", nil)
	cdc.RegisterConcrete(IBCReceiveMsg{}, "cosmos-sdk/IBCReceiveMsg/1", nil)
	cdc.RegisterConcrete(IBCSetMsg{},"cosmos-sdk/IBCSetMsg/1",nil)
	cdc.RegisterConcrete(IBCGetMsg{},"cosmos-sdk/IBCGetMsg/1",nil)
}
