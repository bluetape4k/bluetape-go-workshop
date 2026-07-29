# Code review: issue #114 safe decompression upload

## 범위

- 새 runnable example: `examples/safe-decompression-upload`
- architecture, request sequence, error policy를 위한 새 README diagram
- root README navigation과 focused run instruction
- untrusted compressed upload handling lesson note

## 발견 사항

local review pass에서 P0/P1 finding은 없다.

## 점검

- compressed request body는 `io.ReadAll` 전에 `http.MaxBytesReader`로 제한된다.
- expanded payload는 `compression.DecompressLimit`로 제한된다.
- `compression.ErrDecompressedSizeExceeded`는 `errors.Is`로 계속 관찰 가능하다.
- public HTTP error는 compressed request overflow, decompressed payload overflow,
  malformed compressed bytes, unsupported algorithm, invalid expanded JSON, cancellation을
  구분한다.
- runtime HTTP server는 기본적으로 loopback에 bind하고 non-loopback `HTTP_ADDR` 값을 거부한다.
- README diagram은 PNG로 render되며 boundary, flow, policy concern을 분리한다.

## 잔여 Risk

예제는 bounded decompression과 strict JSON validation에서 의도적으로 멈춘다. malware scanning,
authorization, durable storage, deeper schema governance는 downstream responsibility로
문서화되어 있다.
