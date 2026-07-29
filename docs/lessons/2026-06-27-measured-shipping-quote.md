# Measured shipping quote 예제

Issue: #115

## 결정

실행 가능한 Gin shipping quote API로 `measure` parsing, formatting, derived area/volume,
dimensional weight, billable-weight selection, 안정적인 HTTP error mapping을 설명한다.

## 이유

`measure` package는 caller가 application boundary를 넘어 실제 unit string을 보낼 때 가장
이해하기 쉽다. shipping quote는 unit risk를 구체화한다. dimension과 weight는 metric 또는
imperial 형태로 들어올 수 있지만, application은 raw float와 suffix string을 섞지 않고 typed
value를 비교해야 한다.

예제는 의도적으로 money total 대신 billable measurement를 반환한다. 이렇게 하면 dimensional
math와 currency policy가 분리되고 `measure` lesson이 main reader contract로 남는다.

## 검증 형태

- service test는 metric input, pound를 포함한 imperial dimension, typed incompatible-unit
  error, parse failure, divide-by-zero preservation, declared max-side validation,
  invalid display-unit handling을 assert한다.
- router test는 stable JSON response, malformed JSON handling, invalid-measure
  mapping, incompatible-unit mapping, invalid dimensional divisor mapping,
  max-side request validation, oversize request rejection, health output을 assert한다.
- README diagram은 architecture, request sequence, error policy를 별도 reader question으로
  보여 HTTP behavior를 숨기지 않으면서 measurement boundary를 드러낸다.
