# Issue #59 S3 Floci Storage Code Review

## 범위

- Branch: `feat/issue-59-s3-floci-storage`
- Baseline: `origin/develop`
- Changed area: `examples/s3-floci-storage`, root README pair, S3 README diagram,
  `go.mod`

## 발견 사항

P0=0 P1=0

- P0 finding 없음: 예제는 real cloud credential을 사용하지 않고 external AWS state를 mutate하지
  않으며, smoke coverage는 `BLUETAPE_S3_FLOCI_STORAGE_SMOKE` 뒤에 opt-in으로 둔다.
- P1 finding 없음: 모든 public storage operation은 caller context를 받고, S3 response body는
  close되며, missing object는 `ErrObjectNotFound`로 mapping된다. unsafe key segment는 거부되고
  test는 success, failure, resource cleanup, cancellation-before-call behavior를 다룬다.

## Evidence

- `go test -count=1 ./examples/s3-floci-storage/...`
- `go test -race -count=1 ./examples/s3-floci-storage/...`
- `go run ./examples/s3-floci-storage`
- `xmllint --noout docs/images/readme-diagrams/s3-floci-storage-architecture.svg docs/images/readme-diagrams/s3-floci-storage-sequence.svg`
- both SVG asset에 대해 `~/.local/bin/cairosvg ... -s 2` 실행 후 rendered PNG inspection.

## 잔여 Risk

- smoke test는 Docker가 필요하므로 opt-in이다.
- 예제는 IAM policy design, KMS, lifecycle rule, object lock, replication, alarm, CDN
  delivery를 의도적으로 생략한다. README는 local example이 이를 다룬 척하지 않고 production
  concern으로 명시한다.
- `ListTenantReceipts`는 tenant-prefix listing 한 page를 보여준다. tenant가 S3 page 하나보다
  많은 object를 소유할 수 있는 production repository는 continuation token을 loop해야 한다.
