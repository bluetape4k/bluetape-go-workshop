# Code review: issue #115 measured shipping quote

## 범위

- 새 runnable example: `examples/measured-shipping-quote`
- architecture, request sequence, error policy를 위한 새 README diagram
- root README navigation과 focused run instruction
- `measure` typed-unit boundary handling lesson note

## 발견 사항

local review pass에서 P0/P1 finding은 없다.

## 점검

- caller-facing length string은 `measure.ParseLength`를 통해 built-in `measure` length unit을
  사용한다.
- mass parsing은 built-in `g`, `kg`, `ton`을 유지하면서 realistic shipping input을 위해 local
  `lb`를 추가한다.
- area와 volume은 application code의 untyped multiplication이 아니라 `measure.AreaFromLength`와
  `measure.VolumeFromAreaLength`로 파생한다.
- dimensional weight conversion은 divisor가 divide operation까지 도달하면
  `measure.ErrDivideByZero`를 `errors.Is`로 보존한다.
- public HTTP error code는 invalid request, invalid measurement, incompatible unit,
  invalid dimensional divisor를 구분하며 raw invalid measurement input을 되돌려 주지 않는다.
- runtime HTTP server는 기본적으로 loopback에 bind하고 non-loopback `HTTP_ADDR` 값을 거부한다.
- README diagram은 PNG로 render되며 architecture, sequence, policy concern을 분리한다.

## 잔여 Risk

예제는 carrier money pricing, carrier-specific divisor table, insurance, customs rule을
모델링하지 않는다. 예제가 `measure`에 집중하도록 이들은 downstream policy concern으로
문서화되어 있다.

## P0/P1 Gate

P0=0 P1=0
