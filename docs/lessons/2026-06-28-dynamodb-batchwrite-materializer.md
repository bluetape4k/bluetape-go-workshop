# DynamoDB batch write materializer 예제

Issue: #116

## 결정

더 큰 AWS Floci workflow example 전에 local document-index materializer로
`dynamodb/batchwrite` boundary를 설명한다. 예제는 domain event를 AWS SDK v2
`types.WriteRequest` 값으로 mapping한 뒤 chunking과 `UnprocessedItems` retry를
`batchwrite.WriteAll`에 위임한다.

## 이유

DynamoDB batch write에는 작지만 중요한 protocol이 있다. call당 최대 25개 request,
`UnprocessedItems`를 통한 partial success, retry exhaustion/typed service error/caller
cancellation의 서로 다른 handling이다. 이 protocol을 모든 worker에 넣으면 예제가 장황하고
오류에 취약해진다.

따라서 예제는 item-shape ownership을 application에 두고 DynamoDB batch mechanics에는
bluetape-go를 사용한다. helper를 다시 구현하지 않고도 domain key, attribute name, retry
budget, production handoff policy를 reviewable하게 만든다.

## 검증 형태

- deterministic fake-client test는 event 30개가 `25 + 5` DynamoDB call이 됨을 assert한다.
- retry test는 반환된 `UnprocessedItems`만 다시 submit됨을 assert한다.
- exhaustion test는 `batchwrite.UnprocessedItemsError`를 보존하면서 `ErrRetryExhausted`를
  assert한다.
- cancellation 및 typed AWS service error test는 caller가 ownership failure와
  infrastructure failure를 계속 구분할 수 있음을 assert한다.
- README diagram은 architecture ownership과 retry sequencing을 분리해 설명한다.
