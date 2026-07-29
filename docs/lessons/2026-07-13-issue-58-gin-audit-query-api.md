# Issue #58 Gin Audit Query API Lesson

## 맥락과 결정

0.9.0 workshop track에는 order audit history lesson을 HTTP-shaped로 이어가는 예제가 필요했다.
따라서 이 예제는 query policy를 새 workshop library로 옮기지 않고 released v0.18.0 audit query
model을 Gin으로 노출한다.

search는 JSON body를 가진 POST `/audit/history/search`를 사용한다. 이는 현실적인 URL-length
constraint를 피하고 structured revision/time filter를 둘 공간을 남긴다. metadata는 각 audit
entry와 함께 반환하지만, v0.18.0의 `audit.Query`가 그 contract를 제공하지 않으므로 metadata
filtering은 의도적으로 없다.

## Pagination Contract 경계

service는 reader에게 `limit + 1` entry를 요청한다. 보이지 않는 extra entry는 다음 page 존재를
증명하고 exclusive revision boundary를 제공한다. ascending query는 `next.from_revision`을,
descending query는 `next.to_revision`을 반환한다. 다음 request가 그 값을 포함하므로 전달된
revision이 중복되지 않고 opaque cursor format도 만들지 않는다.

maximum page size는 100이다. 이는 returned/encoded data를 bound할 뿐, in-memory repository의
total work를 bound하지 않는다. 현재 구현은 여전히 stored history를 scan/copy할 수 있다.
durable adapter에는 자체 indexed pagination과 retention design이 필요하다.

## HTTP와 Lifecycle Boundary

JSON boundary는 optional UTF-8 charset과 identity encoding을 가진 `application/json`만 받는다.
oversized, non-UTF-8, duplicate-key, unknown-field, trailing-value body는 거부한다. aggregate
identifier는 작은 ASCII path-safe alphabet으로 제한하고, Gin은 unescaping 없이 raw path를
유지하며, forwarded proxy address는 신뢰하지 않는다.

server는 기본적으로 loopback에 bind한다. remote unauthenticated exposure는 explicit environment
opt-in이 필요하며 demonstration-only mode로 남는다. read, write, header, idle, request,
shutdown timeout은 bounded다. log에는 request body, identifier, metadata, internal error가
아니라 route, method, status, public code, elapsed time, lifecycle event가 들어간다.

## Review 누락과 수리

- 첫 `make ci` run은 package/export comment 18개 누락과 context 없는 HTTP test request 3개를
  드러냈다. 첫 focused lint rerun은 initial report limit 뒤 instance 2개를 더 드러냈다. 모든
  request constructor는 이제 explicit context를 받고, 모든 public symbol에는 유용한 comment가
  있으며, focused lint는 `0 issues`를 보고한다.
- overlength identifier fixture는 원래 NUL-filled byte slice를 사용했다. coverage를 바꾸지 않고
  읽기 쉬운 129-character ASCII value로 교체했다.
- 첫 long-running full gate는 output이 green이었지만 process exit status를 잃었다. proof로
  인정하지 않았고, captured `exit 0` marker와 함께 `make ci`를 처음부터 다시 실행했다.
- `go run` interrupt는 signal handling을 증명하지만 wrapper exit를 모호하게 만들 수 있다.
  그래서 compiled binary를 health, POST search, detail lookup, SIGTERM,
  `shutdown_completed`까지 실행해 exit 0으로 끝나는지 확인했다.

## 결과와 Future Guard

예제는 strict POST query와 exact revision endpoint를 통해 deterministic order history,
paired English/Korean guidance, stable public error, normal/race/live verification을 제공한다.
authentication, authorization, tenant isolation, metadata filtering, durable storage, opaque
cursor, streaming, public compatibility surface를 추가하기 전에는 design을 다시 연다. 이러한
변경은 lesson을 단순 확장하는 것이 아니라 trust, pagination, ownership contract를 바꾼다.
