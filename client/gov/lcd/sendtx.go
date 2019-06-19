package lcd

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/irisnet/irishub/app/v1/gov"
	"github.com/irisnet/irishub/client/context"
	client "github.com/irisnet/irishub/client/gov"
	"github.com/irisnet/irishub/client/utils"
	"github.com/irisnet/irishub/codec"
	sdk "github.com/irisnet/irishub/types"
)

type postProposalReq struct {
	BaseTx         utils.BaseTx   `json:"base_tx"`
	Title          string         `json:"title"`           //  Title of the proposal
	Description    string         `json:"description"`     //  Description of the proposal
	ProposalType   string         `json:"proposal_type"`   //  Type of proposal. Initial set {PlainTextProposal, SoftwareUpgradeProposal}
	Proposer       sdk.AccAddress `json:"proposer"`        //  Address of the proposer
	InitialDeposit string         `json:"initial_deposit"` // Coins to add to the proposal's deposit
	Param          gov.Param      `json:"param"`
	Usage          gov.UsageType  `json:"usage"`
	DestAddress    sdk.AccAddress `json:"dest_address"`
	Percent        sdk.Dec        `json:"percent"`
	Token          token          `json:"token"`
}

type token struct {
	Symbol         string `json:"symbol"`
	SymbolAtSource string `json:"symbol_at_source"`
	Name           string `json:"name"`
	Decimal        uint8  `json:"decimal"`
	SymbolMinAlias string `json:"symbol_min_alias"`
	InitialSupply  uint64 `json:"initial_supply"`
	MaxSupply      uint64 `json:"max_supply"`
	Mintable       bool   `json:"mintable"`
}

type depositReq struct {
	BaseTx    utils.BaseTx   `json:"base_tx"`
	Depositor sdk.AccAddress `json:"depositor"` // Address of the depositor
	Amount    string         `json:"amount"`    // Coins to add to the proposal's deposit
}

type voteReq struct {
	BaseTx utils.BaseTx   `json:"base_tx"`
	Voter  sdk.AccAddress `json:"voter"`  //  address of the voter
	Option string         `json:"option"` //  option from OptionSet chosen by the voter
}

func postProposalHandlerFn(cdc *codec.Codec, cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cliCtx = utils.InitReqCliCtx(cliCtx, r)

		var req postProposalReq
		err := utils.ReadPostBody(w, r, cdc, &req)
		if err != nil {
			return
		}

		baseReq := req.BaseTx.Sanitize()
		if !baseReq.ValidateBasic(w, cliCtx) {
			return
		}

		proposalType, err := gov.ProposalTypeFromString(client.NormalizeProposalType(req.ProposalType))
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		initDepositAmount, err := cliCtx.ParseCoins(req.InitialDeposit)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// create the message
		msg := gov.NewMsgSubmitProposal(req.Title, req.Description, proposalType, req.Proposer, initDepositAmount, gov.Params{req.Param})
		if msg.ProposalType == gov.ProposalTypeTxTaxUsage {
			taxMsg := gov.NewMsgSubmitTaxUsageProposal(msg, req.Usage, req.DestAddress, req.Percent)
			err = msg.ValidateBasic()
			if err != nil {
				utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			utils.SendOrReturnUnsignedTx(w, cliCtx, req.BaseTx, []sdk.Msg{taxMsg})
			return
		}
		if proposalType == gov.ProposalTypeParameterChange {
			if err := client.ValidateParam(req.Param); err != nil {
				utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		if msg.ProposalType == gov.ProposalTypeAddToken {
			token := req.Token
			tokenMsg := gov.NewMsgSubmitAddTokenProposal(msg, token.Symbol, token.SymbolAtSource, token.Name, token.SymbolMinAlias, token.Decimal, token.InitialSupply, token.MaxSupply, token.Mintable)
			if tokenMsg.ValidateBasic() != nil {
				utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
				return
			}
			utils.SendOrReturnUnsignedTx(w, cliCtx, req.BaseTx, []sdk.Msg{tokenMsg})
			return
		}
		err = msg.ValidateBasic()
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		utils.SendOrReturnUnsignedTx(w, cliCtx, req.BaseTx, []sdk.Msg{msg})
	}
}

func depositHandlerFn(cdc *codec.Codec, cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cliCtx = utils.InitReqCliCtx(cliCtx, r)
		vars := mux.Vars(r)
		strProposalID := vars[RestProposalID]

		if len(strProposalID) == 0 {
			utils.WriteErrorResponse(w, http.StatusBadRequest, "proposalId required but not specified")
			return
		}

		proposalID, ok := utils.ParseUint64OrReturnBadRequest(w, strProposalID)
		if !ok {
			return
		}

		var req depositReq
		err := utils.ReadPostBody(w, r, cdc, &req)
		if err != nil {
			return
		}

		baseReq := req.BaseTx.Sanitize()
		if !baseReq.ValidateBasic(w, cliCtx) {
			return
		}

		depositAmount, err := cliCtx.ParseCoins(req.Amount)
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		// create the message
		msg := gov.NewMsgDeposit(req.Depositor, proposalID, depositAmount)
		err = msg.ValidateBasic()
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		utils.SendOrReturnUnsignedTx(w, cliCtx, req.BaseTx, []sdk.Msg{msg})
	}
}

func voteHandlerFn(cdc *codec.Codec, cliCtx context.CLIContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		cliCtx = utils.InitReqCliCtx(cliCtx, r)
		vars := mux.Vars(r)
		strProposalID := vars[RestProposalID]

		if len(strProposalID) == 0 {
			err := errors.New("proposalId required but not specified")
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		proposalID, ok := utils.ParseUint64OrReturnBadRequest(w, strProposalID)
		if !ok {
			return
		}

		var req voteReq
		err := utils.ReadPostBody(w, r, cdc, &req)
		if err != nil {
			return
		}

		baseReq := req.BaseTx.Sanitize()
		if !baseReq.ValidateBasic(w, cliCtx) {
			return
		}

		voteOption, err := gov.VoteOptionFromString(client.NormalizeVoteOption(req.Option))
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		// create the message
		msg := gov.NewMsgVote(req.Voter, proposalID, voteOption)
		err = msg.ValidateBasic()
		if err != nil {
			utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}

		utils.SendOrReturnUnsignedTx(w, cliCtx, req.BaseTx, []sdk.Msg{msg})
	}
}
