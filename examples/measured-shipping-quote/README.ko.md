# measured-shipping-quote

[English](README.md) | [한국어](README.ko.md)

`measure` typed value를 사용하는 Gin shipping quote 예제입니다.

이 예제는 caller가 실제로 입력하는 형태 그대로 parcel dimensions와 weight를 받습니다.
예를 들어 `"40 cm"`, `"12 in"`, `"3.2 kg"`, `"20 lb"` 같은 문자열입니다. Service는
이 문자열을 `measure.Measure[T]` 값으로 parse하고, `measure` compound helper로 area와
volume을 만든 뒤, volume을 dimensional weight로 변환하고 stable JSON quote projection을
반환합니다.

핵심은 carrier pricing이 아닙니다. 핵심은 boundary입니다. Application이 값을 비교하거나,
곱하거나, formatting하기 전에 unit string을 typed measurement로 바꿔야 합니다.

## 시나리오

Warehouse API가 carrier adapter로 parcel을 보내기 전에 quote-like measurement decision을
만들어야 합니다. Caller는 box dimension과 actual weight를 알고 있지만, 입력 단위는 metric과
imperial이 섞일 수 있습니다.

- Domestic workflow는 `width="40 cm"`, `height="30 cm"`, `length="20 cm"`,
  `weight="3.2 kg"`를 보냅니다.
- Imported catalog는 `width="12 in"`, `height="10 in"`, `length="18 in"`,
  `weight="20 lb"`를 보냅니다.

Application은 두 입력을 같은 measured domain model로 변환합니다. Floor area, volume,
dimensional weight, billable weight, 간단한 handling class를 계산합니다. 잘못된 숫자,
호환되지 않는 dimension, invalid dimensional divisor는 `errors.Is`로 확인 가능한 typed
error cause를 유지하고, HTTP response는 stable public error로 유지합니다.

## Architecture

![Measured shipping quote architecture](../../docs/images/readme-diagrams/measured-shipping-quote-architecture.png)

Router는 HTTP boundary를 소유합니다. JSON body size, request binding, public error code가
여기에 있습니다. Service는 measurement boundary를 소유합니다. Parsing, conversion,
derived value, policy decision이 여기에 있습니다. 이 책임 분리는 public response에 raw parse
text를 노출하지 않으면서도 Go code에서는 typed error를 보존하게 해줍니다.

## Request Sequence

![Measured shipping quote sequence](../../docs/images/readme-diagrams/measured-shipping-quote-sequence.png)

1. Client가 measurement string을 `/quotes`로 보냅니다.
2. Router가 JSON envelope를 검증하고 `Service.Quote`를 호출합니다.
3. Service는 length field를 built-in length unit으로 parse하고, mass는 built-in `g`,
   `kg`, `ton`에 local `lb` unit을 더한 shipping registry로 parse합니다.
4. `measure.AreaFromLength`와 `measure.VolumeFromAreaLength`가 typed area와 volume을
   만듭니다.
5. Volume은 `cm^3`로 formatting되고, configured divisor로 dimensional kilogram이 된 뒤
   actual weight와 비교됩니다.
6. Response는 parsed dimensions, derived values, billable weight, billable rule,
   handling policy를 반환합니다.

## Error Policy

![Measured shipping quote policy](../../docs/images/readme-diagrams/measured-shipping-quote-policy.png)

| Boundary | Status | Error code |
|---|---|---|
| Malformed JSON 또는 필수 field 누락 | `400` | `invalid_request` |
| `"many cm"` 같은 non-numeric measurement text | `400` | `invalid_measure` |
| `width="2 kg"` 같은 dimension mismatch | `400` | `incompatible_unit` |
| `dimensional_divisor`가 zero, negative, NaN, infinite | `400` | `invalid_dimensional_divisor` |
| Declared max side가 parsed dimensions보다 작음 | `400` | `invalid_request` |

Service는 `measure.ErrInvalidParse`, `measure.ErrInvalidUnit`,
`measure.ErrDivideByZero`를 wrap합니다. 따라서 test나 상위 layer는 `errors.Is`를 계속
사용할 수 있습니다. HTTP client는 짧고 안정적인 `error_code`만 받고, raw invalid
measurement는 response에 echo되지 않습니다.

## 보여주는 것

- Built-in metric/imperial length unit을 사용하는 `measure.ParseLength`.
- Built-in `g`, `kg`, `ton`을 유지하고 realistic shipping input을 위해 `lb`만 추가한 작은
  mass registry.
- Derived value를 만드는 `measure.AreaFromLength`, `measure.VolumeFromAreaLength`.
- Stable JSON output을 위한 `Measure.Format`, `Measure.In`.
- Actual weight와 dimensional weight 중 billable weight를 고르는 `Measure.Compare`.
- `measure.ErrDivideByZero`를 보존하는 `Measure.DivScalar`.
- Invalid parse, incompatible unit, invalid divisor, request validation failure에 대한 stable
  Gin error mapping.

## 실행

Local measured quote API를 실행합니다.

```bash
go run ./examples/measured-shipping-quote
```

Service는 기본적으로 `127.0.0.1:8104`에서 실행됩니다.

```bash
curl http://127.0.0.1:8104/healthz
curl -s -X POST http://127.0.0.1:8104/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "quote_id":"ship-1001",
    "destination_zone":"kr-seoul",
    "width":"40 cm",
    "height":"30 cm",
    "length":"20 cm",
    "weight":"3.2 kg"
  }' | jq
```

예상 response는 `200 OK`, `billable_rule="dimensional_weight"`, volume `24000 cm^3`,
dimensional weight `4.8 kg`, handling class `parcel`입니다.

Imperial dimension은 inch display를 요청할 수 있고, mass는 kilograms로 반환됩니다.

```bash
curl -s -X POST http://127.0.0.1:8104/quotes \
  -H 'Content-Type: application/json' \
  -d '{
    "quote_id":"ship-1002",
    "destination_zone":"us-west",
    "width":"12 in",
    "height":"10 in",
    "length":"18 in",
    "weight":"20 lb",
    "dimensional_divisor":6000,
    "declared_max_side":"2 ft",
    "preferred_length_unit":"in"
  }' | jq
```

주요 환경 변수:

| 변수 | 기본값 | 용도 |
|---|---|---|
| `HTTP_ADDR` | `127.0.0.1:8104` | Loopback HTTP bind address입니다. Non-loopback bind는 거부합니다. |

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Process liveness only. |
| `POST` | `/quotes` | Measurement를 parse하고 derived shipping quote를 반환합니다. |

## Boundary Notes

- Application boundary에서 unit을 한 번 parse한 뒤, 나머지 계산에는 typed value를 전달합니다.
- Dimensional math와 currency math를 섞지 않습니다. 이 예제는 의도적으로 money total이
  아니라 billable measurement를 반환합니다.
- Public error code는 짧고 안정적으로 유지합니다. 자세한 parse cause는 server log, test,
  typed error chain에 둡니다.
- Unit registry는 product contract의 일부입니다. 여기서 `lb`를 추가한 것은 carrier와
  warehouse data가 pounds로 들어오는 경우가 흔하기 때문입니다.
- Dimensional-weight divisor는 carrier policy마다 달라집니다. Divisor는 policy input으로
  취급하고, quote가 조용히 잘못 계산되지 않도록 zero divisor를 거부합니다.

## 테스트

```bash
go test -count=1 ./examples/measured-shipping-quote/...
go test -race -count=1 ./examples/measured-shipping-quote/...
```
