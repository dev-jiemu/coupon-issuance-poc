# 📠 Coupon Issuance

---

## 구현
본 프로젝트는 아래 총 3개의 RPC Service 를 구현했습니다 :)

1. **CampaignService**
   - `CreateCampaign`: 새로운 쿠폰 캠페인 생성
   - `GetCampaign`: 캠페인 정보 조회 (성공적으로 발행된 쿠폰 코드 포함)

2. **CouponService**
   - `IssueCoupon`: 특정 캠페인에 대한 쿠폰 발행 요청

---

## Stack
- **언어**: Golang (1.23.4 version)
  - 특정 golang 버전을 따로 언급하진 않은듯 해서 임의로 지정했습니다.
- **RPC 프레임워크**: connectrpc (https://connectrpc.com/)
- **프로토콜 버퍼**: Protocol Buffers (proto3)

---

## 프로젝트 구조 ⚙️

```
coupon-issuance-poc/
├── cmd/
│   ├── main/
│   │   ├── main.go               # 애플리케이션 진입점
│   │   └── dev_main.go        
│   ├── proto/                    # 프로토콜 버퍼 정의 파일들
│   │   ├── v1/
│   │   │   ├── campaign.proto
│   │   │   ├── coupon.proto
│   │   │   └── common.proto
│   │   ├── buf.yaml              # buf 구성 파일
│   │   └── buf.gen.yaml          # buf 코드 생성 설정
│   └── test/                     
│       └── load.go               # 종합 테스트 실행 코드
├── pkg/
│   ├── cache/
│   │   └── campaign_manager.go   # 캠페인 및 쿠폰 관리 (메모리 기반)
│   ├── gen/                    
│   │   └── v1/
│   │       ├── *.pb.go       
│   │       └── v1connect/
│   │           ├── campaign.connect.go
│   │           └── coupon.connect.go
│   ├── models/                
│   ├── service/                   # RPC Service 구현체
│   │   ├── campaign_service.go
│   │   └── coupon_service.go
│   └── utils/           
├── go.mod
├── go.sum
└── README.md
```

---

## 구현 세부 사항 ⚒️

### 1) 동시성 처리
문제에서 제시한 초당 500-1000 건의 요청을 처리하는 환경을 구현하기 위해 mutax 를 사용했습니다.
- **이중 계층 mutax 구조** 
  - `CampaignManager`에서 전체 캠페인 맵에 대한 동시 접근을 제어하는 뮤텍스
  - 각 `Campaign` 객체 내부에 해당 캠페인 데이터에 대한 동시 접근을 제어하는 뮤텍스

### 2) 쿠폰 발행 프로세스

전체적인 프로세스는 다음과 같습니다.

1. CreateCampaign : 캠페인 요청 시점에 시작-종료 날짜/최대 생성 count 수를 지정하면 최대 쿠폰수 만큼 Coupon ID 를 지정합니다.
2. IssueCoupon : 발행처리가 안된 번호 중 랜덤으로 하나 가지고 와서 응답으로 주고, 해당 Coupon ID 는 발행상태를 업데이트 합니다.

* GetCampaign 서비스는 정보 조회 역할을 하는 것으로 판단되어 생성한 캠페인의 정보만 return 하는 기능만 담당합니다.

---

### 3) 고려한 엣지 케이스

다음과 같은 엣지 케이스에 대한 대응을 구현해봤습니다. :)

- **캠페인 유효성 검사**: 존재하지 않는 캠페인에 대한 쿠폰 발행 요청 방지
- **쿠폰 소진**: 최대 발행 숫자 또는 이미 다 발행한 상태에서 요청하는 경우 방지
- **시간 제약 조건**: 캠페인 시작, 종료 시간 외 처리에 대한 요청 방지
- **중복 쿠폰 발행 방지**: 동일한 쿠폰 ID가 중복 발행되지 않도록 처리
- **쿠폰 ID 생성**: 캠페인 별로 쿠폰 ID 값을 중복되지 않게 처리

---
## 실행 방법
1. repository clone
```bash
git clone https://github.com/dev-jiemu/coupon-issuance-poc.git
cd coupon-issuance-poc
```

2. 의존성 설치
```bash
go mod tidy
```

3. 서버 실행
```bash
cd cmd
go run main/main.go
```

Default Server Port : `50051`

---
## 테스트 및 검증

다음과 같은 방법으로 기능을 테스트할 수 있습니다
### 단건 테스트 : curl 사용 (HTTP/1.1)

1. **캠페인 생성**
```bash
curl -X POST \
     -H "Content-Type: application/json" \
     -d '{"campaignId":"camp001","startDate":"2025-01-01","expiredDate":"2025-12-31","maxCoupon":1000}' \
     http://localhost:50051/v1.CampaignService/CreateCampaign
```

2. **쿠폰 발행**
```bash
curl -X POST \
     -H "Content-Type: application/json" \
     -d '{"campaignId":"camp001"}' \
     http://localhost:50051/v1.CouponService/IssueCoupon
```

3. **캠페인 정보 조회**
```bash
curl -X POST \
     -H "Content-Type: application/json" \
     -d '{"campaignId":"camp001"}' \
     http://localhost:50051/v1.CampaignService/GetCampaign
  ```

### 종합 테스트
```bash
cd cmd
go run test/load.go -server=http://localhost:50051 -users=100 -campaigns=1 -time=30s -start-date=2025-05-12 -end-date=2025-05-19
```
1. 매개변수 설명
- `server`: Connect RPC 서버 주소 (기본값: http://localhost:50051)
- `users`: 동시 접속 사용자 수 (기본값: 100)
- `campaigns`: 생성할 캠페인 수 (기본값: 1)
- `time`: 테스트 실행 시간 (기본값: 1분, 예: 30s, 5m)
- `start-date`, `end-date`: 캠페인 유효 기간 (YYYY-MM-DD 형식, 미지정시 현재 날짜 기준으로 자동 설정)

2. 테스트 동작 방식

- 지정된 수의 캠페인을 먼저 생성합니다. 테스트를 위해 캠페인 생성시 동시접속 유저의 수 보다 25% 감소된 수치로 쿠폰을 발급합니다. 
- 사용자들은 라운드 로빈 방식으로 캠페인에 할당되어 쿠폰 발급 및 캠페인 조회 요청을 반복합니다.
- 모든 캠페인의 쿠폰이 소진되거나 지정된 테스트 시간이 경과하면 테스트가 종료됩니다.

3. 결과 지표

- 총 요청 수, 성공/실패 비율
- 요청 유형별(캠페인 생성, 쿠폰 발급, 캠페인 조회) 성공/실패 카운트
- 초당 요청 수(RPS)
- 평균 응답 시간
- 쿠폰 소진 캠페인 수


---
## 추가적으로 생각해봐야 할 사항들

### GetCampaign 서비스에 관해서

- 현재 구조상 캠페인을 생성하는 시점에 채번을 다 끝내놓고 있고, 발행 여부와 사용 여부는 별도의 boolean 필드로만 관리하고 있습니다.
그렇기 때문에 현재 구조상 캠페인 정보를 조회하는 시점에서 쿠폰을 발행해도, 캠페인을 생성해도, 정보가 바뀌어도 return 하는 값 자체는 변하지 않겠다 판단이 들어서 mutax 를 따로 적용하지 않았는데요. 만약 쿠폰을 발행하는 시점에 coupon id 채번을 진행해야 한다면, 해당 서비스에도 read mutax 를 적용해줘야겠단 생각이 드네요.


### 이미 종료된 Campaign 에 대한 후처리

- 현재 expired 된 캠페인에 대한 추가 제거 처리에 대한 로직이 별도로 없습니다.. 🥲 메모리 관리를 위해 필요할것 같아요.


### Coupon 발급 관련해서
- 현재 구현에서는 CampaignManager 와 Campaign 접근 시에만 mutax 를 적용한 상태입니다. 확실하게 하려면 개별 Coupon 에도 적용하는게 맞는데요. 현 프로젝트의 크기상 3중 mutax 는 너무 오버헤드가 아닌가 하는 생각이 들어서 제외한 상태입니다.
- 캠페인을 생성하는 시점에 쿠폰을 발급하는게 아니라 쿠폰 발급 요청할 때 채번 하는 방향으로 변경할수도 있을것 같습니다. 그럼 mutax 가 필요할것 같네요.
- 기존 채번 방식(캠페인 생성시 쿠폰채번 먼저 진행)을 그대로 유지할 경우, 채번 양이 많아진다면 고루틴 처리가 필요할것 같습니다.


### 캠페인 생성시 날짜 범위 지정 예외케이스 추가 필요

- 현재로썬 yyyy-mm-dd 포맷으로 들어온다고만 가정했는데, 다른 포맷들이 들어올수도 있을것 같네요.
