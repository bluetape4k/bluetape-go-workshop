# Issue #40 리서치: Work Report와 Failure Policy 예제

## 맥락

- 이슈: `#40 [v0.4.0] Add work report and failure policy example`
- 상위 이슈: `#28 [Epic] v0.4.0 state and workflow examples`
- 대상 마일스톤 주제: v0.4.0 state/workflow examples
- 워크숍 기준선: framework가 lesson의 일부이면 public HTTP API 예제는 Gin을 사용한다.

## 현재 저장소 근거

- `examples/order-lifecycle-state-api`는 이미 Gin API와 안정적인 transition 응답으로
  `state` 패키지를 가르친다.
- `examples/fulfillment-workflow-runner`는 이미 `workflow` runner, request
  cancellation, branch cancellation, report projection을 가르친다.
- `README.md`와 `README.ko.md`는 v0.4.0을 state/workflow examples로 나열하며,
  state API와 fulfillment workflow runner를 이미 포함한다.
- 다음 v0.4.0 공백은 `workreport.Report`와 `workreport.FailurePolicy`가 workflow
  runner의 출력 모양에 그치지 않고 primary lesson이 되는 집중 예제다.

## 라이브러리 근거

`github.com/bluetape4k/bluetape-go@v0.5.1/workreport`에서 확인한 내용은 다음과
같다.

- `FailurePolicy`는 `StopOnFailure`와 `ContinueOnFailure`를 지원한다.
- `Aggregate(name, policy, children...)`의 반환 규칙:
  - 모든 child가 완료되면 `completed`를 반환한다.
  - `StopOnFailure`에서는 첫 non-completed child가 parent status, error, reason을
    결정하고, 복사된 child slice는 해당 failing child까지로 잘린다.
  - `ContinueOnFailure`에서는 모든 child report를 보존하고, child 중 하나라도
    non-completed이면 parent는 `partial`이 된다.
- `Report` constructor는 runtime timestamp를 설정하므로, 예제 HTTP 응답은 안정적인
  DTO로 project하고 timestamp를 생략해야 한다.
- status vocabulary는 `completed`, `failed`, `partial`, `aborted`, `cancelled`다.
  전용 `skipped` status는 없으며, caller-defined skip은 reason이 있는 `aborted`로
  표현할 수 있다.
- 알 수 없는 policy는 `workreport.FailurePolicyError`를 통해
  `ErrUnknownFailurePolicy`를 반환한다.

## 시나리오 결정

operations checklist report용 Gin API인 `examples/operations-report-policy`를
만든다. 이 API는 작은 release-readiness 실행을 모델링한다.

1. `load-catalog-snapshot`은 request가 정상적으로 시작되면 완료된다.
2. `validate-products`는 request input에 따라 완료되거나 실패한다.
3. `notify-partner`는 실패한 첫 시도와 완료된 retry를 함께 포함할 수 있어,
   aggregate가 완전 성공인 것처럼 보이지 않으면서 retry 근거를 드러낸다.
4. `refresh-search-index`는 완료되거나, reason이 있는 `aborted`를 사용해
   caller-skipped로 기록된다.

handler는 request input에서 `StopOnFailure` 또는 `ContinueOnFailure`를 선택하고
`workreport.Aggregate`를 직접 사용한다. 이렇게 하면 예제가 workflow runner mechanics가
아니라 report aggregation semantic에 집중된다.

## 거부한 방향

- durable job engine: v0.4.0 예제 범위 밖이며 package lesson을 persistence
  mechanics 뒤에 숨긴다.
- `workflow.Sequential` 감싸기: 이미 `fulfillment-workflow-runner`가 다룬다. #40은
  `workreport.Aggregate`와 failure policy 동작을 명시적으로 보여 주어야 한다.
- 실패한 첫 시도 뒤 retry를 full success로 보고하기: 보존된 failed attempt는 parent를
  partial로 유지해야 하므로 현재 `workreport` aggregate model에서는 오해를 부른다.
- observability 의존성 추가: 이슈는 observability stack 없이도 log/dashboard에 적합한
  report output을 요구한다.

## 인수 기준 매핑

- 결정적인 work report output: report를 timestamp 없는 안정적인 JSON DTO로 project하고,
  테스트에서 정확한 status/name/summary field를 assert한다.
- retry 동작: nested `notify-partner` attempt report를 포함하고, 실패한 첫 시도와
  완료된 retry가 보존되는지 assert한다.
- skip 동작: 건너뛴 index refresh를 reason이 있는 `aborted`로 노출하고 summary에
  집계한다.
- fail-fast 동작: `StopOnFailure`는 첫 failed child 뒤의 report를 잘라낸다.
- 문서화: English/Korean README는 scenario, architecture, sequence diagram, report
  field, production hardening gap을 포함한다.
