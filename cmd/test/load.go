package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/gen/v1"
	"github.com/dev-jiemu/coupon-issuance-poc/pkg/gen/v1/v1connect"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
)

// 테스트 구성 옵션
var (
	serverAddr   = flag.String("server", "http://localhost:50051", "Connect RPC 서버 주소")
	numUsers     = flag.Int("users", 100, "동시 사용자 수")
	numCampaigns = flag.Int("campaigns", 1, "생성할 캠페인 수 (users보다 작아야 함)")
	testTime     = flag.Duration("time", 1*time.Minute, "테스트 실행 시간")
	startDateStr = flag.String("start-date", "", "캠페인 시작 날짜 (yyyy-mm-dd 형식, 기본값: 현재 날짜)")
	endDateStr   = flag.String("end-date", "", "캠페인 종료 날짜 (yyyy-mm-dd 형식, 기본값: 하루 뒤)")
)

// 결과 측정을 위한 카운터
type Metrics struct {
	// 총 요청 및 결과
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	latencySum      int64 // 마이크로초 단위

	// 요청 유형별 성공/실패 카운터
	couponIssueSuccess    int64
	couponIssueFail       int64
	campaignQuerySuccess  int64
	campaignQueryFail     int64
	campaignCreateSuccess int64
	campaignCreateFail    int64

	// 쿠폰 소진 관련
	exhaustedCampaigns map[string]bool // 소진된 캠페인 ID 추적
	exhaustedMutex     sync.Mutex      // exhaustedCampaigns에 대한 동시성 제어
}

// 테스트 실행기
type LoadTester struct {
	campaignClient v1connect.CampaignServiceClient
	couponClient   v1connect.CouponServiceClient
	metrics        *Metrics
	wg             *sync.WaitGroup
	done           chan struct{}
	campaigns      []string // 생성된 캠페인 ID 목록
	campaignsMutex sync.RWMutex
}

// 새 테스트 실행기 생성
func NewLoadTester(httpClient *http.Client) *LoadTester {
	baseURL := *serverAddr

	return &LoadTester{
		// Connect RPC 클라이언트 생성
		campaignClient: v1connect.NewCampaignServiceClient(httpClient, baseURL),
		couponClient:   v1connect.NewCouponServiceClient(httpClient, baseURL),
		metrics: &Metrics{
			exhaustedCampaigns: make(map[string]bool),
		},
		wg:        &sync.WaitGroup{},
		done:      make(chan struct{}),
		campaigns: make([]string, 0, *numCampaigns),
	}
}

// 가상 사용자 시뮬레이션
func (lt *LoadTester) simulateUser(userId int, campaignId string) {
	defer lt.wg.Done()

	userID := fmt.Sprintf("user-%d", userId)

	for {
		select {
		case <-lt.done:
			return
		default:
			// 쿠폰 발급 요청
			start := time.Now()
			err := lt.issueCoupon(campaignId)
			latency := time.Since(start).Microseconds()

			// 총 요청 수 증가
			atomic.AddInt64(&lt.metrics.totalRequests, 1)

			if err != nil {
				// 쿠폰이 더 이상 없는 경우 처리
				if strings.Contains(err.Error(), "no more available coupon") {
					// 이 캠페인이 이미 소진 처리되었는지 확인
					lt.metrics.exhaustedMutex.Lock()
					if !lt.metrics.exhaustedCampaigns[campaignId] {
						lt.metrics.exhaustedCampaigns[campaignId] = true
						log.Printf("캠페인 %s의 쿠폰이 모두 소진되었습니다. (총 %d/%d 캠페인 소진)",
							campaignId, len(lt.metrics.exhaustedCampaigns), len(lt.campaigns))
					}
					lt.metrics.exhaustedMutex.Unlock()

					// 실패 요청으로 카운트
					atomic.AddInt64(&lt.metrics.failedRequests, 1)
					atomic.AddInt64(&lt.metrics.couponIssueFail, 1)
					return // 이 사용자의 테스트 종료
				}

				log.Printf("사용자 %s 쿠폰 발급 실패: %v", userID, err)
				atomic.AddInt64(&lt.metrics.failedRequests, 1)
				atomic.AddInt64(&lt.metrics.couponIssueFail, 1)
			} else {
				atomic.AddInt64(&lt.metrics.successRequests, 1)
				atomic.AddInt64(&lt.metrics.couponIssueSuccess, 1)
				atomic.AddInt64(&lt.metrics.latencySum, latency)
			}

			// 캠페인 정보 조회
			start = time.Now()
			err = lt.getCampaign(campaignId)
			latency = time.Since(start).Microseconds()

			// 총 요청 수 증가
			atomic.AddInt64(&lt.metrics.totalRequests, 1)

			if err != nil {
				log.Printf("사용자 %s 캠페인 조회 실패: %v", userID, err)
				atomic.AddInt64(&lt.metrics.failedRequests, 1)
				atomic.AddInt64(&lt.metrics.campaignQueryFail, 1)
			} else {
				atomic.AddInt64(&lt.metrics.successRequests, 1)
				atomic.AddInt64(&lt.metrics.campaignQuerySuccess, 1)
				atomic.AddInt64(&lt.metrics.latencySum, latency)
			}

			// 잠시 대기 후 다음 요청 실행
			sleep := 100 + rand.Intn(200)
			time.Sleep(time.Duration(sleep) * time.Millisecond)
		}
	}
}

// 캠페인 생성 요청
func (lt *LoadTester) createCampaign(campaignNum int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 시작 날짜와 종료 날짜 설정
	var startDate, expiredDate string

	// 시작 날짜 설정
	if *startDateStr == "" {
		// 기본값: 현재 날짜
		startDate = time.Now().Add(-1 * time.Hour).Format("2006-01-02") // 시작 시간을 1시간 이전으로 설정하여 유효성 검사 통과
	} else {
		startDate = *startDateStr
	}

	// 종료 날짜 설정
	if *endDateStr == "" {
		// 기본값: 하루 뒤
		expiredDate = time.Now().Add(24 * time.Hour).Format("2006-01-02")
	} else {
		expiredDate = *endDateStr
	}

	// 고유한 캠페인 ID 생성
	campaignId := fmt.Sprintf("campaign-%d-%d", campaignNum, time.Now().UnixNano())

	// MaxCoupon을 users 수보다 적게 설정 (75%로 설정)
	maxCoupon := int64(*numUsers * 3 / 4)
	if maxCoupon < 1 {
		maxCoupon = 1 // 최소 1개 이상
	}

	req := connect.NewRequest(&v1.CreateCampaignReq{
		CampaignId:  campaignId,
		StartDate:   startDate,
		ExpiredDate: expiredDate,
		MaxCoupon:   maxCoupon,
	})

	// 총 요청 수 증가
	atomic.AddInt64(&lt.metrics.totalRequests, 1)

	resp, err := lt.campaignClient.CreateCampaign(ctx, req)
	if err != nil {
		atomic.AddInt64(&lt.metrics.failedRequests, 1)
		atomic.AddInt64(&lt.metrics.campaignCreateFail, 1)
		return "", err
	}

	// 성공 여부 확인
	if !resp.Msg.Result.Success {
		atomic.AddInt64(&lt.metrics.failedRequests, 1)
		atomic.AddInt64(&lt.metrics.campaignCreateFail, 1)
		return "", fmt.Errorf("API 오류: %s (코드: %s)", resp.Msg.Result.Message, resp.Msg.Result.ErrorCode)
	}

	atomic.AddInt64(&lt.metrics.successRequests, 1)
	atomic.AddInt64(&lt.metrics.campaignCreateSuccess, 1)

	log.Printf("캠페인 생성 완료 (ID: %s, 최대 쿠폰 수: %d)", campaignId, maxCoupon)
	return campaignId, nil
}

// 쿠폰 발급 요청
func (lt *LoadTester) issueCoupon(campaignId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := connect.NewRequest(&v1.IssueCouponReq{
		CampaignId: campaignId,
	})

	resp, err := lt.couponClient.IssueCoupon(ctx, req)
	if err != nil {
		return err
	}

	// 성공 여부 확인
	if !resp.Msg.Result.Success {
		return fmt.Errorf("API 오류: %s (코드: %s)", resp.Msg.Result.Message, resp.Msg.Result.ErrorCode)
	}

	return nil
}

// 캠페인 조회 요청
func (lt *LoadTester) getCampaign(campaignId string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := connect.NewRequest(&v1.GetCampaignReq{
		CampaignId: campaignId,
	})

	resp, err := lt.campaignClient.GetCampaign(ctx, req)
	if err != nil {
		return err
	}

	// 성공 여부 확인
	if !resp.Msg.Result.Success {
		return fmt.Errorf("API 오류: %s (코드: %s)", resp.Msg.Result.Message, resp.Msg.Result.ErrorCode)
	}

	return nil
}

// 테스트 실행
func (lt *LoadTester) RunTest() {
	startTime := time.Now()
	log.Printf("부하 테스트 시작: %d개의 캠페인, %d명의 동시 사용자, %v 동안 실행", *numCampaigns, *numUsers, *testTime)

	// 캠페인 수가 사용자 수보다 많지 않도록 확인
	if *numCampaigns > *numUsers {
		log.Println("경고: 캠페인 수가 사용자 수보다 많습니다. 캠페인 수를 사용자 수로 제한합니다.")
		*numCampaigns = *numUsers
	}

	// 캠페인 먼저 생성
	for i := 0; i < *numCampaigns; i++ {
		campaignId, err := lt.createCampaign(i)
		if err != nil {
			log.Printf("캠페인 생성 실패: %v", err)
			continue
		}

		// 생성된 캠페인 저장
		lt.campaigns = append(lt.campaigns, campaignId)
	}

	// 캠페인이 하나도 생성되지 않은 경우 종료
	if len(lt.campaigns) == 0 {
		log.Println("오류: 캠페인이 하나도 생성되지 않았습니다. 테스트를 종료합니다.")
		return
	}

	// 사용자 시작 (캠페인 수에 따라 캠페인을 분배)
	for i := 0; i < *numUsers; i++ {
		// 캠페인 ID 선택 (라운드 로빈 방식)
		campaignIndex := i % len(lt.campaigns)
		campaignId := lt.campaigns[campaignIndex]

		lt.wg.Add(1)
		go lt.simulateUser(i, campaignId)

		// 사용자 점진적 시작 (모든 사용자가 동시에 시작하면 초기 스파이크 발생)
		time.Sleep(10 * time.Millisecond)
	}

	// 진행 상황 모니터링
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	testEndTime := startTime.Add(*testTime)

	for {
		select {
		case <-ticker.C:
			total := atomic.LoadInt64(&lt.metrics.totalRequests)
			success := atomic.LoadInt64(&lt.metrics.successRequests)
			failed := atomic.LoadInt64(&lt.metrics.failedRequests)

			// 소진된 캠페인 수 확인
			lt.metrics.exhaustedMutex.Lock()
			exhaustedCount := len(lt.metrics.exhaustedCampaigns)
			lt.metrics.exhaustedMutex.Unlock()

			elapsed := time.Since(startTime).Seconds()
			rps := float64(total) / elapsed

			log.Printf("진행: %.1f%% | 요청: %d | 성공: %d | 실패: %d | RPS: %.1f | 쿠폰 소진 캠페인: %d/%d",
				time.Since(startTime).Seconds()/testTime.Seconds()*100,
				total, success, failed, rps, exhaustedCount, len(lt.campaigns))

			// 모든 캠페인의 쿠폰이 소진되면 테스트 종료
			if exhaustedCount >= len(lt.campaigns) {
				log.Println("모든 캠페인의 쿠폰이 모두 소진되었습니다. 테스트를 종료합니다.")
				close(lt.done)
				log.Println("모든 사용자 종료 대기 중...")
				lt.wg.Wait()
				lt.printResults(time.Since(startTime).Seconds())
				return
			}

		case <-time.After(time.Until(testEndTime)):
			// 테스트 종료 시간 도달
			close(lt.done)
			log.Println("테스트 시간 완료, 모든 사용자 종료 대기 중...")
			lt.wg.Wait()

			lt.printResults(time.Since(startTime).Seconds())
			return
		}
	}
}

// 테스트 결과 출력
func (lt *LoadTester) printResults(elapsedSeconds float64) {
	total := atomic.LoadInt64(&lt.metrics.totalRequests)
	success := atomic.LoadInt64(&lt.metrics.successRequests)
	failed := atomic.LoadInt64(&lt.metrics.failedRequests)

	couponSuccess := atomic.LoadInt64(&lt.metrics.couponIssueSuccess)
	couponFail := atomic.LoadInt64(&lt.metrics.couponIssueFail)
	campaignQuerySuccess := atomic.LoadInt64(&lt.metrics.campaignQuerySuccess)
	campaignQueryFail := atomic.LoadInt64(&lt.metrics.campaignQueryFail)
	campaignCreateSuccess := atomic.LoadInt64(&lt.metrics.campaignCreateSuccess)
	campaignCreateFail := atomic.LoadInt64(&lt.metrics.campaignCreateFail)

	var avgLatency int64
	if success > 0 {
		avgLatency = atomic.LoadInt64(&lt.metrics.latencySum) / success
	}

	rps := float64(total) / elapsedSeconds

	// 소진된 캠페인 수 확인
	lt.metrics.exhaustedMutex.Lock()
	exhaustedCount := len(lt.metrics.exhaustedCampaigns)
	lt.metrics.exhaustedMutex.Unlock()

	fmt.Println("\n========== Test Result ==========")
	fmt.Printf("총 실행 시간: %.2f초\n", elapsedSeconds)
	fmt.Printf("총 요청 수: %d\n", total)
	fmt.Printf("성공 요청: %d (%.2f%%)\n", success, float64(success)/float64(total)*100)
	fmt.Printf("실패 요청: %d (%.2f%%)\n", failed, float64(failed)/float64(total)*100)
	fmt.Printf("초당 요청 수(RPS): %.2f\n", rps)
	fmt.Printf("평균 응답 시간: %d µs\n", avgLatency)
	fmt.Printf("쿠폰 소진 캠페인 수: %d/%d\n", exhaustedCount, len(lt.campaigns))
	fmt.Println("\n--- 요청 유형별 성공/실패 ---")
	fmt.Printf("캠페인 생성: %d 성공, %d 실패\n", campaignCreateSuccess, campaignCreateFail)
	fmt.Printf("쿠폰 발급: %d 성공, %d 실패\n", couponSuccess, couponFail)
	fmt.Printf("캠페인 조회: %d 성공, %d 실패\n", campaignQuerySuccess, campaignQueryFail)
	fmt.Println("====================================")
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	// 기본값 설정: 캠페인 수가 지정되지 않은 경우 1로 설정
	if flag.NFlag() > 0 && *numCampaigns <= 0 {
		*numCampaigns = 1
	}

	// HTTP 클라이언트 생성
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 테스트 실행
	tester := NewLoadTester(httpClient)
	tester.RunTest()
}
