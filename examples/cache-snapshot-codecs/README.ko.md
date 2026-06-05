# cache-snapshot-codecs

[English](README.md) | [한국어](README.ko.md)

`serialization`과 `compression`을 함께 사용하는 versioned cache snapshot
예제입니다.

이 예제는 상품 catalog snapshot을 JSON payload로 직렬화하고, 버전이 있는
envelope에 넣은 뒤 `gzip` 또는 `zstd`로 압축합니다. 테스트에는 압축 크기
비교가 포함되어 있지만, 실제 알고리즘 선택은 workload별 측정으로 결정해야
합니다.

## Scenario

![Data codec and cleanup flow](../../docs/images/readme-diagrams/data-codec-cleanup-flow.png)

cache가 portable snapshot 형식과 작은 payload를 동시에 필요로 할 때 사용할 수
있는 흐름입니다. versioned serializer는 payload shape을 명시적으로 보존하고,
compressor는 별도로 측정하고 교체할 수 있는 선택지로 둡니다.

## What It Demonstrates

- JSON payload를 감싼 `serialization.VersionedSerializer`.
- 호환성을 우선하는 `compression.Gzip()`.
- 내부 저장 속도를 우선하는 `compression.Zstd()`.
- 큰 snapshot을 위한 stream decompression.
- 깨진 압축 bytes와 깨진 serialized payload를 분리해서 검증하는 실패 케이스.

## Run

```bash
go test -count=1 ./examples/cache-snapshot-codecs/...
```
