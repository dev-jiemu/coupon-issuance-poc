package cache

import (
	"errors"
	"fmt"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/models"
	"sync"
	"time"
)

type Campaign struct {
	CampaignId           string
	StartDate            time.Time
	ExpiredDate          time.Time
	MaxCoupons           int
	UnPublishedCouponIds []string // 발행 안된 coupon id 관리용
	Coupons              map[string]*models.Coupon
	mutex                sync.RWMutex
}

type CampaignManager struct {
	campaigns map[string]*Campaign
	mutex     sync.RWMutex
}

func NewCampaignManager() *CampaignManager {
	fmt.Printf("Create Campaign Manager ** ")
	return &CampaignManager{
		campaigns: make(map[string]*Campaign),
	}
}

func (v *CampaignManager) CreateCampaign(id string, start, end time.Time, maxCoupon int) error {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	if _, exists := v.campaigns[id]; exists {
		return errors.New("campaign already exists")
	}

	campaign := &Campaign{
		CampaignId:           id,
		StartDate:            start,
		ExpiredDate:          end,
		MaxCoupons:           maxCoupon,
		UnPublishedCouponIds: make([]string, 0, maxCoupon),
		Coupons:              make(map[string]*models.Coupon, maxCoupon),
	}

	// 미리 쿠폰 ID는 생성해둠 : 나중에 발급요청 할때 발급유무 변경
	for i := 0; i < maxCoupon; i++ {
		couponId := "" // TODO : unique ID Created

		coupon := &models.Coupon{
			CouponId:    couponId,
			StartDate:   start,
			ExpiredDate: end,
			PublishYn:   false,
			UseYn:       false,
		}

		campaign.Coupons[couponId] = coupon
		campaign.UnPublishedCouponIds = append(campaign.UnPublishedCouponIds, couponId)
	}

	v.campaigns[id] = campaign

	return nil
}

func (v *CampaignManager) PublishCoupon(campaignId string) (*models.Coupon, error) {
	v.mutex.RLock()
	campaign, exists := v.campaigns[campaignId]
	v.mutex.RUnlock()

	if !exists {
		return nil, errors.New("campaign is not exists")
	}

	campaign.mutex.Lock()
	defer campaign.mutex.Unlock()

	// 요청 시점 확인
	now := time.Now()
	if now.Before(campaign.StartDate) || now.After(campaign.ExpiredDate) {
		return nil, errors.New("campaign not valid at this time")
	}

	if len(campaign.UnPublishedCouponIds) == 0 {
		return nil, errors.New("no more available coupon")
	}

	// 발행처리
	lastIdx := len(campaign.UnPublishedCouponIds) - 1
	couponId := campaign.UnPublishedCouponIds[lastIdx]
	campaign.UnPublishedCouponIds = campaign.UnPublishedCouponIds[:lastIdx]

	coupon := campaign.Coupons[couponId]
	coupon.PublishYn = true

	return coupon, nil
}

func (v *CampaignManager) UseCoupon(campaignId, couponId string) error {
	v.mutex.RLock()
	campaign, exists := v.campaigns[campaignId]
	v.mutex.RUnlock()

	if !exists {
		return errors.New("campaign is not exists")
	}

	campaign.mutex.Lock()
	defer campaign.mutex.Unlock()

	coupon, exists := campaign.Coupons[couponId]
	if !exists {
		return errors.New("coupon is not exists")
	}

	// 발행 안된 쿠폰 사용금지
	if !coupon.PublishYn {
		return errors.New("coupon is not published")
	}

	// 이미 사용된 쿠폰이면 에러처리
	if coupon.UseYn {
		return errors.New("coupon is already used")
	}

	// startDate 보다 이전이거나 expiredDate 이후면 에러처리
	now := time.Now()
	if now.Before(coupon.StartDate) || now.After(coupon.ExpiredDate) {
		return errors.New("coupon not valid at this time")
	}

	coupon.UseYn = true

	return nil
}

// TODO : 캠페인 정보 조회 (발급 성공한 쿠폰만 모아서 return)
func (v *CampaignManager) GetCampaignInfo(campaignId string) error {

	return nil
}
