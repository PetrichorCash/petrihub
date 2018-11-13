package init

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	tmcli "github.com/tendermint/tendermint/libs/cli"
	"github.com/tendermint/tendermint/libs/common"
	"io/ioutil"
	"os"
	"path/filepath"
	stakecmd "github.com/irisnet/irishub/client/stake/cli"
	"github.com/irisnet/irishub/client/stake/cli"
	"github.com/irisnet/irishub/app"
	"github.com/irisnet/irishub/client"
	signcmd "github.com/irisnet/irishub/client/bank/cli"
	authcmd "github.com/cosmos/cosmos-sdk/x/auth/client/cli"
)

const (
	defaultAmount                  = "100iris"
	defaultCommissionRate          = "0.1"
	defaultCommissionMaxRate       = "0.2"
	defaultCommissionMaxChangeRate = "0.01"
)

// GenTxCmd builds the gaiad gentx command.
// nolint: errcheck
func GenTxCmd(ctx *server.Context, cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gentx",
		Short: "Generate a genesis tx carrying a self delegation",
		Long: fmt.Sprintf(`This command is an alias of the 'gaiad tx create-validator' command'.

It creates a genesis piece carrying a self delegation with the
following delegation and commission default parameters:

	delegation amount:           %s
	commission rate:             %s
	commission max rate:         %s
	commission max change rate:  %s
`, defaultAmount, defaultCommissionRate, defaultCommissionMaxRate, defaultCommissionMaxChangeRate),
		RunE: func(cmd *cobra.Command, args []string) error {

			config := ctx.Config
			config.SetRoot(viper.GetString(tmcli.HomeFlag))
			nodeID, valPubKey, err := InitializeNodeValidatorFiles(ctx.Config)
			if err != nil {
				return err
			}
			ip := viper.GetString(stakecmd.FlagIP)
			if ip == "" {
				ip, err = server.ExternalIP()
				if err != nil {
					return err
				}
			}
			genDoc, err := loadGenesisDoc(cdc, config.GenesisFile())
			if err != nil {
				return err
			}

			// Read --pubkey, if empty take it from priv_validator.json
			if valPubKeyString := viper.GetString(cli.FlagPubKey); valPubKeyString != "" {
				valPubKey, err = sdk.GetConsPubKeyBech32(valPubKeyString)
				if err != nil {
					return err
				}
			}
			// Run gaiad tx create-validator
			prepareFlagsForTxCreateValidator(config, nodeID, ip, genDoc.ChainID, valPubKey)
			createValidatorCmd := stakecmd.GetCmdCreateValidator(cdc)

			w, err := ioutil.TempFile("", "gentx")
			if err != nil {
				return err
			}
			unsignedGenTxFilename := w.Name()
			defer os.Remove(unsignedGenTxFilename)
			os.Stdout = w
			if err = createValidatorCmd.RunE(nil, args); err != nil {
				return err
			}
			w.Close()

			prepareFlagsForTxSign()
			signCmd := signcmd.GetSignCommand(cdc, authcmd.GetAccountDecoder(cdc))
			if w, err = prepareOutputFile(config.RootDir, nodeID); err != nil {
				return err
			}
			os.Stdout = w
			return signCmd.RunE(nil, []string{unsignedGenTxFilename})
		},
	}

	cmd.Flags().String(tmcli.HomeFlag, app.DefaultNodeHome, "node's home directory")
	cmd.Flags().String(flagClientHome, app.DefaultCLIHome, "client's home directory")
	cmd.Flags().String(client.FlagName, "", "name of private key with which to sign the gentx")
	cmd.Flags().String(stakecmd.FlagIP,"",fmt.Sprintf("Node's public IP. It takes effect only when used in combination with --%s", stakecmd.FlagGenesisFormat))
	cmd.Flags().AddFlagSet(stakecmd.FsCommissionCreate)
	cmd.Flags().AddFlagSet(stakecmd.FsAmount)
	cmd.Flags().AddFlagSet(stakecmd.FsPk)
	cmd.MarkFlagRequired(client.FlagName)
	return cmd
}

func prepareFlagsForTxCreateValidator(config *cfg.Config, nodeID, ip, chainID string,
	valPubKey crypto.PubKey) {
	viper.Set(tmcli.HomeFlag, viper.GetString(flagClientHome)) // --home
	viper.Set(client.FlagChainID, chainID)
	viper.Set(client.FlagFrom, viper.GetString(client.FlagName))   // --from
	viper.Set(stakecmd.FlagNodeID, nodeID)                              // --node-id
	viper.Set(stakecmd.FlagIP, ip)                                      // --ip
	viper.Set(stakecmd.FlagPubKey, sdk.MustBech32ifyConsPub(valPubKey)) // --pubkey
	viper.Set(stakecmd.FlagGenesisFormat, true)                         // --genesis-format
	viper.Set(stakecmd.FlagMoniker, config.Moniker)                     // --moniker
	if config.Moniker == "" {
		viper.Set(stakecmd.FlagMoniker, viper.GetString(client.FlagName))
	}
	if viper.GetString(stakecmd.FlagAmount) == "" {
		viper.Set(stakecmd.FlagAmount, defaultAmount)
	}
	if viper.GetString(stakecmd.FlagCommissionRate) == "" {
		viper.Set(stakecmd.FlagCommissionRate, defaultCommissionRate)
	}
	if viper.GetString(stakecmd.FlagCommissionMaxRate) == "" {
		viper.Set(stakecmd.FlagCommissionMaxRate, defaultCommissionMaxRate)
	}
	if viper.GetString(stakecmd.FlagCommissionMaxChangeRate) == "" {
		viper.Set(stakecmd.FlagCommissionMaxChangeRate, defaultCommissionMaxChangeRate)
	}
}

func prepareFlagsForTxSign() {
	viper.Set("offline", true)
}

func prepareOutputFile(rootDir, nodeID string) (w *os.File, err error) {
	writePath := filepath.Join(rootDir, "config", "gentx")
	if err = common.EnsureDir(writePath, 0700); err != nil {
		return
	}
	filename := filepath.Join(writePath, fmt.Sprintf("gentx-%v.json", nodeID))
	return os.Create(filename)
}
