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
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
)

// 테스트 구성 옵션
var (
	serverAddr = flag.String("server", "http://localhost:8080", "Connect RPC 서버 주소")
	numUsers   = flag.Int("users", 100, "동시 사용자 수")
	testTime   = flag.Duration("time", 1*time.Minute, "테스트 실행 시간")
)

// 결과 측정을 위한 카운터
type Metrics struct {
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	latencySum      int64 // 마이크로초 단위
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
		metrics:        &Metrics{},
		wg:             &sync.WaitGroup{},
		done:           make(chan struct{}),
		campaigns:      make([]string, 0, 1000),
	}
}

// 가상 사용자 시뮬레이션
func (lt *LoadTester) simulateUser(userId int) {
	defer lt.wg.Done()

	userID := fmt.Sprintf("user-%d", userId)

	for {
		select {
		case <-lt.done:
			return
		default:
			// 1. 캠페인 생성
			campaignId, err := lt.createCampaign(userID)
			if err != nil {
				log.Printf("사용자 %s 캠페인 생성 실패: %v", userID, err)
				atomic.AddInt64(&lt.metrics.totalRequests, 1)
				atomic.AddInt64(&lt.metrics.failedRequests, 1)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			atomic.AddInt64(&lt.metrics.totalRequests, 1)
			atomic.AddInt64(&lt.metrics.successRequests, 1)

			// 생성된 캠페인 저장
			lt.campaignsMutex.Lock()
			lt.campaigns = append(lt.campaigns, campaignId)
			if len(lt.campaigns) > 1000 {
				lt.campaigns = lt.campaigns[len(lt.campaigns)-1000:]
			}
			lt.campaignsMutex.Unlock()

			// 2. 쿠폰 발급 요청
			start := time.Now()
			err = lt.issueCoupon(campaignId)
			latency := time.Since(start).Microseconds()

			if err != nil {
				log.Printf("사용자 %s 쿠폰 발급 실패: %v", userID, err)
				atomic.AddInt64(&lt.metrics.totalRequests, 1)
				atomic.AddInt64(&lt.metrics.failedRequests, 1)
			} else {
				atomic.AddInt64(&lt.metrics.totalRequests, 1)
				atomic.AddInt64(&lt.metrics.successRequests, 1)
				atomic.AddInt64(&lt.metrics.latencySum, latency)
			}

			// 3. 캠페인 정보 조회
			start = time.Now()
			err = lt.getCampaign(campaignId)
			latency = time.Since(start).Microseconds()

			if err != nil {
				log.Printf("사용자 %s 캠페인 조회 실패: %v", userID, err)
				atomic.AddInt64(&lt.metrics.totalRequests, 1)
				atomic.AddInt64(&lt.metrics.failedRequests, 1)
			} else {
				atomic.AddInt64(&lt.metrics.totalRequests, 1)
				atomic.AddInt64(&lt.metrics.successRequests, 1)
				atomic.AddInt64(&lt.metrics.latencySum, latency)
			}

			// 잠시 대기 후 다음 요청 실행
			sleep := 100 + rand.Intn(200)
			time.Sleep(time.Duration(sleep) * time.Millisecond)
		}
	}
}

// 캠페인 생성 요청
func (lt *LoadTester) createCampaign(userId string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 현재 시간과 만료 시간 (24시간 뒤) 설정
	startDate := time.Now().Format(time.RFC3339)
	expiredDate := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	// 고유한 캠페인 ID 생성
	campaignId := fmt.Sprintf("campaign-%s-%d", userId, time.Now().UnixNano())

	req := connect.NewRequest(&v1.CreateCampaignReq{
		CampaignId:  campaignId,
		StartDate:   startDate,
		ExpiredDate: expiredDate,
		MaxCoupon:   1000,
	})

	resp, err := lt.campaignClient.CreateCampaign(ctx, req)
	if err != nil {
		return "", err
	}

	// 성공 여부 확인
	if !resp.Msg.Result.Success {
		return "", fmt.Errorf("API 오류: %s (코드: %s)", resp.Msg.Result.Message, resp.Msg.Result.ErrorCode)
	}

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
	log.Printf("부하 테스트 시작: %d명의 동시 사용자, %v 동안 실행", *numUsers, *testTime)

	// 사용자 시작
	for i := 0; i < *numUsers; i++ {
		lt.wg.Add(1)
		go lt.simulateUser(i)

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

			elapsed := time.Since(startTime).Seconds()
			rps := float64(total) / elapsed

			log.Printf("진행: %.1f%% | 요청: %d | 성공: %d | 실패: %d | RPS: %.1f",
				time.Since(startTime).Seconds()/testTime.Seconds()*100,
				total, success, failed, rps)

		case <-time.After(time.Until(testEndTime)):
			// 테스트 종료
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

	var avgLatency int64
	if success > 0 {
		avgLatency = atomic.LoadInt64(&lt.metrics.latencySum) / success
	}

	rps := float64(total) / elapsedSeconds

	fmt.Println("\n========== 테스트 결과 ==========")
	fmt.Printf("총 실행 시간: %.2f초\n", elapsedSeconds)
	fmt.Printf("총 요청 수: %d\n", total)
	fmt.Printf("성공 요청: %d (%.2f%%)\n", success, float64(success)/float64(total)*100)
	fmt.Printf("실패 요청: %d (%.2f%%)\n", failed, float64(failed)/float64(total)*100)
	fmt.Printf("초당 요청 수(RPS): %.2f\n", rps)
	fmt.Printf("평균 응답 시간: %d µs\n", avgLatency)
	fmt.Println("==============================")
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	// HTTP 클라이언트 생성
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// 테스트 실행
	tester := NewLoadTester(httpClient)
	tester.RunTest()
}
