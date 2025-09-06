package model

type SaveBondsRequest struct {
	InvestData []byte `json:"invest"`
	RedeemData []byte `json:"redeem"`
}
