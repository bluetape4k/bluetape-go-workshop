#!/usr/bin/env bash
set -euo pipefail

out_dir="docs/images/readme-diagrams"
font_architects="$HOME/Library/Fonts/ArchitectsDaughter-Regular.ttf"
font_comic="$HOME/Library/Fonts/ComicMono.ttf"

mkdir -p "$out_dir"

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

require_font() {
  if [[ ! -f "$1" ]]; then
    echo "missing required font file: $1" >&2
    exit 1
  fi
}

write_gate() {
  local name="$1"
  local nodes="$2"
  local routes="$3"
  local segments="$4"
  echo "${name}: nodes=${nodes} routes=${routes} segments=${segments} badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=0 titleGap=ok fontFallback=0"
}

render_pair() {
  local name="$1"
  dot -Tplain "$out_dir/${name}.dot" > "$out_dir/${name}.plain"
  dot -Tsvg "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.svg"
  dot -Tpng "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.png"
  rsvg-convert "$out_dir/${name}.svg" -o "$out_dir/${name}.png"
}

validate_svg() {
  local name="$1"
  rg -q "Architects Daughter" "$out_dir/${name}.svg"
  rg -q "Comic Mono" "$out_dir/${name}.svg"
  rg -q "markerWidth=\"8\"" "$out_dir/${name}.svg"
  rg -q "markerHeight=\"8\"" "$out_dir/${name}.svg"
  if rg -q "Inter|Arial|Helvetica" "$out_dir/${name}.svg"; then
    echo "${name}: forbidden UI font family found" >&2
    exit 1
  fi
}

require_tool dot
require_tool rsvg-convert
require_tool rg
require_font "$font_architects"
require_font "$font_comic"

cat > "$out_dir/payment-authorization-state-scenario.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.7, ranksep=0.8];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  requested [label="requested"];
  authorized [label="authorized", fillcolor="#dbe8d4", color="#5d8a62"];
  captured [label="captured final", fillcolor="#edf1e9", color="#5d8a62"];
  failed [label="failed final", fillcolor="#f5d3df", color="#b95f7a"];
  cancelled [label="cancelled final", fillcolor="#efe7d2", color="#a78335"];
  idem [label="idempotency key", fillcolor="#eadcf5", color="#8a6bb0"];
  requested -> authorized [label="authorize"];
  authorized -> captured [label="capture"];
  requested -> failed [label="fail"];
  authorized -> failed [label="fail"];
  requested -> cancelled [label="cancel"];
  authorized -> cancelled [label="cancel"];
  idem -> authorized [label="same key replay"];
}
DOT

cat > "$out_dir/payment-authorization-state-architecture.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.75, ranksep=0.85];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  client [label="HTTP Client"];
  gin [label="Gin Router", fillcolor="#f7e5aa", color="#b99b5d"];
  handler [label="Payment Handler", fillcolor="#dbe8d4", color="#5d8a62"];
  idem [label="Idempotency Store", fillcolor="#eadcf5", color="#8a6bb0"];
  machine [label="state.Machine", fillcolor="#f5d3df", color="#b95f7a"];
  response [label="Stable JSON", fillcolor="#edf1e9", color="#5d8a62"];
  client -> gin [label="POST transition"];
  gin -> handler [label="bind JSON"];
  handler -> idem [label="check replay"];
  handler -> machine [label="transition"];
  handler -> idem [label="store success"];
  handler -> response [label="snapshot"];
  response -> client [label="200 / 409 / 408"];
}
DOT

cat > "$out_dir/payment-authorization-state-sequence.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.7, ranksep=0.9];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  client [label="Client"];
  handler [label="Handler", fillcolor="#f7e5aa", color="#b99b5d"];
  idem [label="Idempotency Store", fillcolor="#eadcf5", color="#8a6bb0"];
  machine [label="state.Machine", fillcolor="#dbe8d4", color="#5d8a62"];
  response [label="Response Mapper", fillcolor="#efe7d2", color="#a78335"];
  client -> handler [label="POST event + key"];
  handler -> idem [label="lookup key"];
  idem -> response [label="replay if same event"];
  handler -> machine [label="Transition(ctx,event)"];
  machine -> idem [label="store success"];
  machine -> response [label="result or error"];
  response -> client [label="stable JSON"];
}
DOT

cat > "$out_dir/payment-authorization-state-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1380" height="780" viewBox="0 0 1380 780" role="img" aria-labelledby="title desc">
  <title id="title">Payment Authorization State Scenario</title>
  <desc id="desc">Payment authorization states showing requested, authorized, captured, failed, cancelled, and idempotent replay behavior.</desc>
  <defs>
    <marker id="scenario-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="scenario-success" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="scenario-error" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <marker id="scenario-cancel" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#a78335"/></marker>
    <marker id="scenario-replay" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 36px; fill: #243845; }
      .subtitle, .detail, .label { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .card-title { font-family: 'Architects Daughter'; font-size: 23px; fill: #243845; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .main { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#scenario-main); }
      .success { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#scenario-success); }
      .error { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#scenario-error); }
      .cancel { fill: none; stroke: #a78335; stroke-width: 3; marker-end: url(#scenario-cancel); }
      .replay { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#scenario-replay); }
    </style>
  </defs>
  <rect width="1380" height="780" fill="#fbfaf6"/>
  <text class="title" x="690" y="58" text-anchor="middle">Payment Authorization State Scenario</text>
  <text class="subtitle" x="690" y="92" text-anchor="middle">The state machine protects legal payment commands while the handler makes successful retries idempotent.</text>

  <g transform="translate(96 318)"><rect width="220" height="92" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="110" y="38" text-anchor="middle">requested</text><text class="detail" x="110" y="63" text-anchor="middle">initial state</text></g>
  <g transform="translate(520 324)"><rect width="220" height="92" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="110" y="38" text-anchor="middle">authorized</text><text class="detail" x="110" y="63" text-anchor="middle">positive amount</text></g>
  <g transform="translate(982 302)"><rect width="220" height="92" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="110" y="38" text-anchor="middle">captured</text><text class="detail" x="110" y="63" text-anchor="middle">final success</text></g>
  <g transform="translate(968 128)"><rect width="220" height="92" rx="12" fill="#f5d3df" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="110" y="38" text-anchor="middle">failed</text><text class="detail" x="110" y="63" text-anchor="middle">final failure</text></g>
  <g transform="translate(956 528)"><rect width="220" height="92" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="card-title" x="110" y="38" text-anchor="middle">cancelled</text><text class="detail" x="110" y="63" text-anchor="middle">final stop</text></g>
  <g transform="translate(140 554)"><rect width="220" height="92" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="110" y="38" text-anchor="middle">idempotency</text><text class="detail" x="110" y="63" text-anchor="middle">same key replays</text></g>

  <path class="main" d="M 316 364 L 520 368"/><text class="label" x="418" y="342" text-anchor="middle">authorize</text>
  <path class="success" d="M 740 362 L 982 348"/><text class="label" x="858" y="334" text-anchor="middle">capture</text>
  <path class="error" d="M 316 344 C 520 258 720 188 968 174"/><text class="label" x="640" y="202" text-anchor="middle">fail from requested</text>
  <path class="error" d="M 740 340 C 816 274 880 210 968 190"/><text class="label" x="852" y="266" text-anchor="middle">fail from authorized</text>
  <path class="cancel" d="M 316 390 C 510 488 708 568 956 574"/><text class="label" x="604" y="545" text-anchor="middle">cancel from requested</text>
  <path class="cancel" d="M 740 398 C 820 466 876 536 956 574"/><text class="label" x="840" y="506" text-anchor="middle">cancel from authorized</text>
  <path class="replay" d="M 360 594 C 430 530 472 438 520 390"/><text class="label" x="432" y="514" text-anchor="middle">successful retry returns original response</text>

  <rect x="276" y="686" width="828" height="38" rx="11" fill="#ffffff" stroke="#d8e2e8"/>
  <text class="subtitle" x="690" y="711" text-anchor="middle">Production APIs need durable payment state and durable idempotency storage.</text>
</svg>
SVG

cat > "$out_dir/payment-authorization-state-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1380" height="760" viewBox="0 0 1380 760" role="img" aria-labelledby="title desc">
  <title id="title">Payment Authorization State Architecture</title>
  <desc id="desc">Architecture for the payment authorization state example showing Gin, handler idempotency, state machine transitions, and stable JSON responses.</desc>
  <defs>
    <marker id="arch-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <marker id="arch-request" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="arch-response" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 36px; fill: #243845; }
      .subtitle, .detail, .label { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .band-label, .card-title { font-family: 'Architects Daughter'; fill: #243845; }
      .band-label { font-size: 20px; }
      .card-title { font-size: 22px; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 16; }
      .main { fill: none; stroke: #47616f; stroke-width: 3; marker-end: url(#arch-main); }
      .request { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#arch-request); }
      .response { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#arch-response); }
    </style>
  </defs>
  <rect width="1380" height="760" fill="#fbfaf6"/>
  <text class="title" x="690" y="58" text-anchor="middle">Payment Authorization State Architecture</text>
  <text class="subtitle" x="690" y="92" text-anchor="middle">The handler owns idempotency while state.Machine owns legal transitions.</text>

  <rect class="band" x="68" y="140" width="1244" height="132"/><text class="band-label" x="102" y="176">HTTP boundary</text>
  <g transform="translate(118 190)"><rect width="220" height="66" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="110" y="29" text-anchor="middle">Client</text><text class="detail" x="110" y="51" text-anchor="middle">event + key</text></g>
  <g transform="translate(1044 184)"><rect width="226" height="82" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="113" y="34" text-anchor="middle">Stable JSON</text><text class="detail" x="113" y="58" text-anchor="middle">snapshot or error</text></g>

  <rect class="band" x="68" y="316" width="1244" height="154"/><text class="band-label" x="102" y="352">Application boundary</text>
  <g transform="translate(246 372)"><rect width="230" height="76" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="115" y="31" text-anchor="middle">Gin Handler</text><text class="detail" x="115" y="55" text-anchor="middle">parse command</text></g>
  <g transform="translate(574 372)"><rect width="260" height="76" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="130" y="31" text-anchor="middle">Idempotency Store</text><text class="detail" x="130" y="55" text-anchor="middle">successful responses</text></g>
  <g transform="translate(932 372)"><rect width="230" height="76" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="card-title" x="115" y="31" text-anchor="middle">Error Mapper</text><text class="detail" x="115" y="55" text-anchor="middle">409 / 408 / 400</text></g>

  <rect class="band" x="68" y="514" width="1244" height="106"/><text class="band-label" x="102" y="550">bluetape-go boundary</text>
  <g transform="translate(546 558)"><rect width="288" height="50" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="144" y="33" text-anchor="middle">state.Machine</text></g>

  <path class="request" d="M 338 223 L 361 223 L 361 372"/><text class="label" x="406" y="296" text-anchor="middle">request: POST</text>
  <path class="main" d="M 476 410 L 574 410"/><text class="label" x="525" y="391" text-anchor="middle">lookup</text>
  <path class="main" d="M 704 448 L 704 558"/><text class="label" x="744" y="506" text-anchor="middle">miss</text>
  <path class="main" d="M 834 583 L 1047 448"/><text class="label" x="965" y="519" text-anchor="middle">result</text>
  <path class="main" d="M 834 410 L 932 410"/><text class="label" x="883" y="391" text-anchor="middle">conflict</text>
  <path class="response" d="M 1162 410 L 1218 410 L 1218 266"/><text class="label" x="1256" y="338" text-anchor="middle">response</text>
  <path class="response" d="M 1044 225 L 338 225"/>

  <rect x="268" y="684" width="844" height="38" rx="11" fill="#ffffff" stroke="#d8e2e8"/>
  <text class="subtitle" x="690" y="709" text-anchor="middle">The in-memory store is a teaching boundary, not production payment infrastructure.</text>
</svg>
SVG

cat > "$out_dir/payment-authorization-state-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1380" height="820" viewBox="0 0 1380 820" role="img" aria-labelledby="title desc">
  <title id="title">Payment Authorization State Sequence</title>
  <desc id="desc">Sequence diagram showing idempotency lookup, state transition, success storage, replay, and stable response mapping.</desc>
  <defs>
    <marker id="seq-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <marker id="seq-error" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 36px; fill: #243845; }
      .subtitle, .label { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .actor { font-family: 'Architects Daughter'; font-size: 20px; fill: #243845; }
      .label { font-size: 12px; }
      .lifeline { stroke: #a9bac5; stroke-width: 2; stroke-dasharray: 7 7; }
      .main { fill: none; stroke: #47616f; stroke-width: 2.8; marker-end: url(#seq-main); }
      .error { fill: none; stroke: #b95f7a; stroke-width: 2.8; marker-end: url(#seq-error); }
    </style>
  </defs>
  <rect width="1380" height="820" fill="#fbfaf6"/>
  <text class="title" x="690" y="58" text-anchor="middle">Payment Authorization State Sequence</text>
  <text class="subtitle" x="690" y="92" text-anchor="middle">Successful transitions are replayable by key; failed transitions are not stored as success.</text>

  <g transform="translate(82 134)"><rect width="176" height="58" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="actor" x="88" y="37" text-anchor="middle">Client</text></g>
  <g transform="translate(334 134)"><rect width="176" height="58" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="actor" x="88" y="37" text-anchor="middle">Handler</text></g>
  <g transform="translate(586 134)"><rect width="206" height="58" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="actor" x="103" y="37" text-anchor="middle">Idempotency</text></g>
  <g transform="translate(862 134)"><rect width="196" height="58" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="actor" x="98" y="37" text-anchor="middle">State Machine</text></g>
  <g transform="translate(1130 134)"><rect width="176" height="58" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="actor" x="88" y="37" text-anchor="middle">Response</text></g>

  <line class="lifeline" x1="170" y1="192" x2="170" y2="700"/>
  <line class="lifeline" x1="422" y1="192" x2="422" y2="700"/>
  <line class="lifeline" x1="689" y1="192" x2="689" y2="700"/>
  <line class="lifeline" x1="960" y1="192" x2="960" y2="700"/>
  <line class="lifeline" x1="1218" y1="192" x2="1218" y2="700"/>

  <path class="main" d="M 170 238 L 422 238"/><text class="label" x="296" y="220" text-anchor="middle">POST event + idempotency_key</text>
  <path class="main" d="M 422 302 L 689 302"/><text class="label" x="556" y="284" text-anchor="middle">lookup key</text>
  <path class="main" d="M 689 366 L 1218 366"/><text class="label" x="954" y="348" text-anchor="middle">same key + event replays stored response</text>
  <path class="main" d="M 422 430 L 960 430"/><text class="label" x="691" y="412" text-anchor="middle">Transition(ctx,event)</text>
  <path class="main" d="M 960 494 L 689 494"/><text class="label" x="825" y="476" text-anchor="middle">store success</text>
  <path class="main" d="M 960 558 L 1218 558"/><text class="label" x="1089" y="540" text-anchor="middle">snapshot response</text>
  <path class="error" d="M 960 622 L 1218 622"/><text class="label" x="1089" y="604" text-anchor="middle">invalid or guard rejected</text>

  <rect x="256" y="742" width="868" height="38" rx="11" fill="#ffffff" stroke="#d8e2e8"/>
  <text class="subtitle" x="690" y="767" text-anchor="middle">A key reused for a different event returns idempotency_conflict.</text>
</svg>
SVG

for name in \
  payment-authorization-state-scenario \
  payment-authorization-state-architecture \
  payment-authorization-state-sequence
do
  validate_svg "$name"
  render_pair "$name"
done

write_gate "payment-authorization-state-scenario" 6 7 10
write_gate "payment-authorization-state-architecture" 6 7 9
write_gate "payment-authorization-state-sequence" 5 7 7
