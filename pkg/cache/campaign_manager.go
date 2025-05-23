package cache

import (
	"errors"
	"fmt"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/models"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/utils"
	"log"
	"sync"
	"time"
)

type Campaign struct {
	CampaignId           string
	StartDate            time.Time
	ExpiredDate          time.Time
	MaxCoupons           int64
	UnPublishedCouponIds []string // 발행 안된 coupon id 관리용
	Coupons              map[string]*models.Coupon
	mutex                sync.RWMutex
}

type CampaignInfo struct {
	CampaignId   string
	StartDate    string
	ExpiredDate  string
	AllCouponIds []string
}

type CampaignManager struct {
	campaigns map[string]*Campaign
	mutex     sync.RWMutex
}

var Manager *CampaignManager

func NewCampaignManager() *CampaignManager {
	fmt.Printf("Create Campaign Manager ** \n")
	return &CampaignManager{
		campaigns: make(map[string]*Campaign),
	}
}

func (v *CampaignManager) CreateCampaign(id string, start, end time.Time, maxCoupon int64) error {
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
	// 이렇게 하면 campaign_id 내에서는 중복을 체크하지만 campaign_id 끼리의 쿠폰 ID 는 겹칠 수도 있다는 단점이 있음...
	existingCouponIDs := make(map[string]bool)
	generatedCount := int64(0)

	// 500 ~ 1000건 정도 라고 했으니까 이정돈 for문 써도 상관은 없는데... 더 많은 양의 생성이 필요하다면 고루틴 써야할듯함
	for generatedCount < maxCoupon {
		couponId, err := utils.GenerateCouponCode(10)
		if err != nil {
			return fmt.Errorf("failed to generate coupon ID: %w", err)
		}

		fmt.Printf("generated coupon ID: %s \n", couponId)

		if existingCouponIDs[couponId] { // 중복이면 다시 만들기
			continue
		}

		existingCouponIDs[couponId] = true

		coupon := &models.Coupon{
			CouponId:    couponId,
			StartDate:   start,
			ExpiredDate: end,
			PublishYn:   false,
			UseYn:       false,
		}

		campaign.Coupons[couponId] = coupon
		campaign.UnPublishedCouponIds = append(campaign.UnPublishedCouponIds, couponId)
		generatedCount++
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

	startDateKST := campaign.StartDate.In(time.Local)
	expiredDateKST := campaign.ExpiredDate.In(time.Local)

	log.Printf("현재 시간: %v (UTC: %v)", now, now.UTC())
	log.Printf("캠페인 시작 시간(KST): %v, 종료 시간(KST): %v", startDateKST, expiredDateKST)
	log.Printf("시작 시간 이전 여부: %v, 종료 시간 이후 여부: %v", now.Before(startDateKST), now.After(expiredDateKST))

	// KST로 변환된 시간으로 비교
	if now.Before(startDateKST) || now.After(expiredDateKST) {
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
	startDateKST := coupon.StartDate.In(time.Local)
	expiredDateKST := coupon.ExpiredDate.In(time.Local)

	log.Printf("캠페인 시작 시간(KST): %v, 종료 시간(KST): %v", startDateKST, expiredDateKST)
	log.Printf("시작 시간 이전 여부: %v, 종료 시간 이후 여부: %v", now.Before(startDateKST), now.After(expiredDateKST))

	// KST로 변환된 시간으로 비교
	if now.Before(startDateKST) || now.After(expiredDateKST) {
		return errors.New("coupon not valid at this time")
	}

	coupon.UseYn = true

	return nil
}

// GetCampaignInfo : 캠페인 등록 시점에 쿠폰을 만드는게 아니라, 캠페인 시작 시점에 쿠폰이 실시간으로 바뀐다면 mutax 필요할듯
// 지금으로썬 그저 조회만 하는 역할에 가까워서 mutax 뺌
func (v *CampaignManager) GetCampaignInfo(campaignId string) (*CampaignInfo, error) {
	campaign, exists := v.campaigns[campaignId]
	if !exists {
		return nil, errors.New("campaign is not exists")
	}

	ret := &CampaignInfo{}
	ret.CampaignId = campaign.CampaignId
	ret.StartDate = campaign.StartDate.Format("2006-01-02 15:04:05")
	ret.ExpiredDate = campaign.ExpiredDate.Format("2006-01-02 15:04:05")

	coupons := make([]string, 0, campaign.MaxCoupons)

	for couponId, _ := range campaign.Coupons {
		coupons = append(coupons, couponId)
	}

	ret.AllCouponIds = coupons

	return ret, nil
}
