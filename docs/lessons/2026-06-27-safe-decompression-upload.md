# Safe decompression upload 예제

Issue: #114

## 결정

raw `gzip` 또는 `zstd` byte를 받는 실행 가능한 Gin upload API에서
`compression.DecompressLimit`를 사용한다. strict JSON validation은 compressed byte limit와
expanded byte limit가 모두 통과한 뒤에만 수행한다.

## 이유

기존 cache snapshot example은 trusted internal data의 compression tradeoff를 보여준다. 이
예제는 untrusted HTTP boundary를 다룬다. 작은 compressed request가 큰 in-memory payload로
확장될 수 있기 때문이다. router는 compressed transport byte에 대한 `http.MaxBytesReader`를
소유하고, service는 expanded byte에 대한 `compression.DecompressLimit`와 typed error
preservation을 소유한다.

## 검증 형태

- service test는 valid gzip/zstd payload, malformed compressed byte, unsupported
  algorithm, invalid expanded JSON, cancellation,
  `compression.ErrDecompressedSizeExceeded`의 `errors.Is` preservation을 assert한다.
- router test는 accepted upload, malformed compressed payload, compressed request
  overflow, decompressed payload overflow, unsupported algorithm, canceled request
  context, health, limit inspection에 대한 stable status/error-code mapping을 assert한다.
- README diagram은 boundary architecture, request sequence, error policy를 분리해
  compressed limit와 decompressed limit가 서로 다른 resource를 보호한다는 점을 보여준다.
