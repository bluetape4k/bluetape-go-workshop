# S3-SQS-DynamoDB Document Workflow Lesson

Issue #66은 앞선 S3, SQS, DynamoDB lesson을 조합하는 AWS/Floci integration example이다.
application-shaped로 유지한다. workflow는 object key convention, event shape, idempotency
state, ack/retry decision을 소유하고, AWS SDK client, Floci endpoint, credential, IAM,
encryption, DLQ policy는 caller-owned로 남는다.

예제에 diagram이 두 개 있으면 reader contract를 더 쉽게 따라갈 수 있다.

- application boundary, workflow-owned contract, caller-owned AWS/Floci resource를
  분리하는 architecture view.
- submit, success, duplicate, transient retry outcome을 시간 순서로 보여주는 sequence view.

test는 happy path보다 더 많은 것을 증명해야 한다. 이 workflow의 minimum deterministic suite는
SQS send 전 S3 put, processing 시 S3 body close, DynamoDB conditional write shape, duplicate
ack, transient retry visibility, context cancellation, unsafe key rejection, forged SQS event
consistency를 다룬다.

opt-in Floci smoke test는 S3, SQS, DynamoDB가 활성화된 local AWS-compatible container 하나를
시작한 뒤 serial로 실행해야 한다. 이는 real AWS credential이나 account state 없이 SDK request
compatibility를 증명한다.

Diagram QA lesson: SVG만 보지 말고 CairoSVG render 뒤 full-size PNG를 inspect한다. 첫
architecture pass에는 retry-visible note를 가로지르는 connector와 margin이 좁은 AWS card
label이 있었다. 둘 다 rendered PNG에서만 보였다.

Follow-up diagram QA lesson: sequence-named asset은 lifeline과 arrow만 포함하면 되는 것이
아니라 local best-practices sequence family와 시각적으로 맞아야 한다. PNG를 승인하기 전에
participant header, activation bar, pill label, dashed `alt`/`else` region, 충분한 row height를
사용한다. architecture connector는 각 operation을 고유 corridor에 둔다. state-write line이
object-store line과 좁은 corridor를 공유하거나 layer border에 붙지 않게 한다.
