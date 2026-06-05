# invitation-codecs

[English](README.md)

`codec`를 사용해 invitation link, callback state, partner reference를 다루는
예제입니다.

이 예제는 짧고 URL 친화적인 invitation token에는 Base62, callback state에는
Base64URL, support-facing diagnostic reference에는 Hex를 사용합니다. 이것들은
encoding이지 encryption이 아니므로, 별도 보안 설계 없이 secret을 넣으면 안
됩니다.

## Scenario

![Data codec and cleanup flow](../../docs/images/readme-diagrams/data-codec-cleanup-flow.png)

값이 URL, callback, support boundary를 지나야 하지만 사람이 다루기 어려운
형태가 되면 안 되는 상황을 보여줍니다. 각 codec은 좁은 책임을 가집니다:
Base62는 compact token, Base64URL은 callback state, Hex는 diagnostic reference를
담당합니다.

## What It Demonstrates

- `+`, `/`, padding 문자가 없는 URL-safe invitation link.
- Base64URL을 통한 callback state round trip.
- decoding 이후 callback state shape 검증.
- raw ID를 직접 노출하지 않는 support-facing external reference.
- encoding과 security의 경계.

## Run

```bash
go test -count=1 ./examples/invitation-codecs/...
```
