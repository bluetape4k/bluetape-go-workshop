# Issue #119: Multilingual Language Routing Lesson

## 맥락

workshop에는 `textsearch/language` evidence를 explicit route로 바꾸되 language detection을
authorization, compliance, certainty로 취급하지 않는 application-shaped example이 필요했다.
또한 lazy/preloaded lifecycle choice와 safe shared detector reuse를 보여줘야 했다.

## 결정

English/Korean/Japanese/Chinese detector 하나를 사용하고 application policy는 internal `Router`에
둔다. English와 Korean은 `moderation`을 공유한다. Japanese는 `japanese-tokenization` 전에 Kana를
요구한다. Chinese, mixed, short, unknown 및 기타 uncertain evidence는 `manual-review`로 fail
closed한다. review reason은 하나의 stable order를 가지며 returned evidence는 caller-owned다.

default confidence는 `0.70`으로 유지한다. fallback은 threshold `1.0`으로 별도 시연한다. 그러면
uncertain result를 만들기 위해 production-like default를 약화하지 않는다. `--preload`는
lifecycle/config metadata만 바꾸며 decision은 절대 바꾸지 않는다.

## 의외였던 Evidence

원래 low-confidence fixture 후보는 `bluetape-go` v0.18.0에서 default threshold보다 높았다.
따라서 durable test는 universal detector accuracy claim이 아니라
`detected confidence < configured threshold` 관계를 assert한다. 현재 fixed fixture는 pinned
version에서 `0.9821181320051728`을 보고하지만, 이 숫자는 API guarantee가 아니라 output evidence다.

ready/release gate는 bounded first-use pressure, exact completion, cross-round equality, race
safety를 증명할 수 있다. model evaluation이 내부적으로 overlap됐는지는 증명할 수 없으므로 test는
그 주장을 의도적으로 하지 않는다.

## 결과

deterministic CLI는 fixed request 7개와 별도 low-confidence policy check에 대해 confidence
4개를 내림차순으로, ISO metadata, script hint, UTF-8 byte section, route, ordered review reason을
보고한다. 영어/한국어 README pair는 lifecycle cost, supported route, caller-owned
redaction/logging/access-control duty를 설명한다.

## 검증

새로 성공한 명령:

```bash
go test -count=1 ./examples/multilingual-language-routing/...
go test -race -count=1 ./examples/multilingual-language-routing/...
go run ./examples/multilingual-language-routing
go run ./examples/multilingual-language-routing --preload
make fmt-check
make tidy-check
make vet
make lint
make ci
git diff --check origin/develop...HEAD
```

## Review 누락과 Future Guard

첫 구현은 package test를 통과했지만 best-effort stderr write가 return value를 명시적으로 처리하지
않아 repository lint에 실패했다. 향후 CLI example은 CLI boundary가 green이 되는 즉시 repository
lint gate를 실행하되, 최종 proof로 `make ci`는 계속 유지해야 한다.

detector model이나 fixture가 바뀌면 default policy를 보존하고 fixed evidence를 의도적으로
review한다. historical output을 그대로 두기 위해 threshold를 느슨하게 하거나 fail-closed reason을
제거하거나 exact confidence를 accuracy promise로 문서화하지 않는다.
