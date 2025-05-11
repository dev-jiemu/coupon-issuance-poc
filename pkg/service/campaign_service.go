package service

import (
	"context"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/cache"
	"log"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/dev-jiemu/coupon-issuance-poc/pkg/gen/v1"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/gen/v1/v1connect"
)

type CampaignServer struct{}

// NewCampaignServer creates a new campaign server
func NewCampaignServer() v1connect.CampaignServiceHandler {
	return &CampaignServer{}
}

func (s *CampaignServer) CreateCampaign(context context.Context, req *connect.Request[v1.CreateCampaignReq]) (*connect.Response[v1.CreateCampaignRes], error) {
	log.Printf("CreateCampaign called with campaignId: %s \n", req.Msg.CampaignId)

	campaignRes := &v1.CreateCampaignRes{
		Result: &v1.BaseResponse{
			Success: true,
			Message: "",
		},
	}

	// 날짜포맷 : yyyy-mm-dd 로 들어온다는 가정하에
	// TODO : 날짜포맷 예외케이스 개선 필요
	start, startErr := time.Parse("2006-01-02", req.Msg.StartDate)
	if startErr != nil {
		log.Printf("CreateCampaign failed with error: %v \n", startErr)
		campaignRes.Result.Success = false
		campaignRes.Result.Message = startErr.Error()
		return connect.NewResponse(campaignRes), startErr
	}

	expired, endErr := time.Parse("2006-01-02", req.Msg.ExpiredDate)
	if endErr != nil {
		log.Printf("CreateCampaign failed with error: %v \n", endErr)
		campaignRes.Result.Success = false
		campaignRes.Result.Message = endErr.Error()
		return connect.NewResponse(campaignRes), endErr
	}

	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	expiredDate := time.Date(expired.Year(), expired.Month(), expired.Day(), 23, 59, 59, 0, expired.Location())

	err := cache.Manager.CreateCampaign(req.Msg.CampaignId, startDate, expiredDate, req.Msg.MaxCoupon)
	if err != nil {
		log.Printf("CreateCampaign failed with error: %v \n", err)
		campaignRes.Result.Success = false
		campaignRes.Result.Message = err.Error()
	}

	log.Printf("CreateCampaign result: %v \n", campaignRes)
	return connect.NewResponse(campaignRes), nil
}

func (s *CampaignServer) GetCampaign(context context.Context, req *connect.Request[v1.GetCampaignReq]) (*connect.Response[v1.GetCampaignRes], error) {
	log.Printf("GetCampaign called with campaignId: %s \n", req.Msg.CampaignId)

	campaignRes := &v1.GetCampaignRes{
		Result: &v1.BaseResponse{
			Success: true,
			Message: "",
		},
		Info: &v1.CampaignInfo{
			CampaignId: req.Msg.CampaignId,
		},
	}

	coupons, err := cache.Manager.GetCampaignInfo(req.Msg.CampaignId)
	if err != nil {
		log.Printf("GetCampaign failed with error: %v \n", err)
		campaignRes.Result.Success = false
		campaignRes.Result.Message = err.Error()
		return connect.NewResponse(campaignRes), err
	}

	campaignRes.Info.CampaignId = coupons.CampaignId
	campaignRes.Info.StartDate = coupons.StartDate
	campaignRes.Info.ExpiredDate = coupons.ExpiredDate
	campaignRes.Info.AllCouponIds = coupons.AllCouponIds

	log.Printf("GetCampaign result: %v \n", campaignRes)
	return connect.NewResponse(campaignRes), nil
}
