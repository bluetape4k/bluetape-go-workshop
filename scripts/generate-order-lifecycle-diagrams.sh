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

cat > "$out_dir/order-lifecycle-state-api-scenario.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.65, ranksep=0.8];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  draft [label="Draft"];
  submitted [label="Submitted", fillcolor="#f7e5aa", color="#b99b5d"];
  paid [label="Paid", fillcolor="#dbe8d4", color="#5d8a62"];
  packed [label="Packed", fillcolor="#eadcf5", color="#8a6bb0"];
  shipped [label="Shipped\nfinal", fillcolor="#edf1e9", color="#5d8a62"];
  cancelled [label="Cancelled\nfinal", fillcolor="#f5d3df", color="#b95f7a"];
  draft -> submitted [label="submit"];
  submitted -> paid [label="pay guard"];
  paid -> packed [label="pack"];
  packed -> shipped [label="ship"];
  draft -> cancelled [label="cancel", color="#b95f7a", fontcolor="#8d4c63"];
  submitted -> cancelled [label="cancel", color="#b95f7a", fontcolor="#8d4c63"];
  paid -> cancelled [label="cancel", color="#b95f7a", fontcolor="#8d4c63"];
}
DOT

cat > "$out_dir/order-lifecycle-state-api-architecture.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.75, ranksep=0.85];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  client [label="HTTP Client"];
  gin [label="Gin Router", fillcolor="#f7e5aa", color="#b99b5d"];
  handlers [label="Order Handlers", fillcolor="#dbe8d4", color="#5d8a62"];
  machine [label="state.Machine", fillcolor="#eadcf5", color="#8a6bb0"];
  guard [label="Payment Guard", fillcolor="#f5d3df", color="#b95f7a"];
  errors [label="JSON Error Mapper", fillcolor="#efe7d2", color="#a78335"];
  client -> gin [label="GET/POST"];
  gin -> handlers [label="route"];
  handlers -> machine [label="State, Transition"];
  machine -> guard [label="pay"];
  guard -> machine [label="allow/reject"];
  machine -> errors [label="sentinel errors", color="#a78335", fontcolor="#7a6128"];
  handlers -> client [label="snapshot/response", color="#4f8b63", fontcolor="#3f704f"];
  errors -> client [label="400/409/408", color="#b95f7a", fontcolor="#8d4c63"];
}
DOT

cat > "$out_dir/order-lifecycle-state-api-sequence.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.7, ranksep=0.9];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  client [label="Client"];
  handler [label="Gin Handler", fillcolor="#f7e5aa", color="#b99b5d"];
  machine [label="state.Machine", fillcolor="#eadcf5", color="#8a6bb0"];
  guard [label="Pay Guard", fillcolor="#f5d3df", color="#b95f7a"];
  mapper [label="Response Mapper", fillcolor="#dbe8d4", color="#5d8a62"];
  client -> handler [label="POST submit"];
  handler -> machine [label="Transition(submit)"];
  machine -> mapper [label="draft -> submitted", color="#4f8b63", fontcolor="#3f704f"];
  client -> handler [label="GET pay/can"];
  handler -> machine [label="CanTransition(pay)"];
  machine -> guard [label="evaluate total"];
  guard -> mapper [label="allowed or guard error"];
  mapper -> client [label="200 or 409", color="#b95f7a", fontcolor="#8d4c63"];
}
DOT

cat > "$out_dir/order-lifecycle-state-api-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1480" height="820" viewBox="0 0 1480 820" role="img" aria-labelledby="title desc">
  <title id="title">Order Lifecycle Scenario</title>
  <desc id="desc">Scenario flow for the order lifecycle state API showing the happy path, cancellation path, payment guard, final states, and invalid command handling.</desc>
  <defs>
    <marker id="scenario-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <marker id="scenario-success" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#4f8b63"/></marker>
    <marker id="scenario-cancel" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #243845; }
      .subtitle, .detail, .label { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .card-title { font-family: 'Architects Daughter'; font-size: 24px; fill: #243845; }
      .detail { font-size: 14px; }
      .label { font-size: 13px; }
      .main { fill: none; stroke: #47616f; stroke-width: 3; marker-end: url(#scenario-main); }
      .success { fill: none; stroke: #4f8b63; stroke-width: 3; marker-end: url(#scenario-success); }
      .cancel { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#scenario-cancel); }
    </style>
  </defs>
  <rect width="1480" height="820" fill="#fbfaf6"/>
  <text class="title" x="740" y="58" text-anchor="middle">Order Lifecycle Scenario</text>
  <text class="subtitle" x="740" y="92" text-anchor="middle">A single order accepts explicit commands; state.Machine guards impossible transitions and concurrent races.</text>

  <g transform="translate(80 178)"><rect width="210" height="116" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="105" y="47" text-anchor="middle">Draft</text><text class="detail" x="105" y="76" text-anchor="middle">created order</text><text class="detail" x="105" y="98" text-anchor="middle">submit or cancel</text></g>
  <g transform="translate(370 178)"><rect width="230" height="116" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="115" y="47" text-anchor="middle">Submitted</text><text class="detail" x="115" y="76" text-anchor="middle">payment is allowed</text><text class="detail" x="115" y="98" text-anchor="middle">guard checks total</text></g>
  <g transform="translate(690 178)"><rect width="210" height="116" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="105" y="47" text-anchor="middle">Paid</text><text class="detail" x="105" y="76" text-anchor="middle">pack or cancel</text><text class="detail" x="105" y="98" text-anchor="middle">fulfillment owns next</text></g>
  <g transform="translate(980 178)"><rect width="210" height="116" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="105" y="47" text-anchor="middle">Packed</text><text class="detail" x="105" y="76" text-anchor="middle">parcel ready</text><text class="detail" x="105" y="98" text-anchor="middle">ship command only</text></g>
  <g transform="translate(1250 178)"><rect width="170" height="116" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="85" y="47" text-anchor="middle">Shipped</text><text class="detail" x="85" y="76" text-anchor="middle">final state</text><text class="detail" x="85" y="98" text-anchor="middle">no more events</text></g>

  <path class="main" d="M 290 236 L 370 236"/><text class="label" x="330" y="216" text-anchor="middle">submit</text>
  <path class="main" d="M 600 236 L 690 236"/><text class="label" x="645" y="216" text-anchor="middle">pay</text>
  <path class="main" d="M 900 236 L 980 236"/><text class="label" x="940" y="216" text-anchor="middle">pack</text>
  <path class="success" d="M 1190 236 L 1250 236"/><text class="label" x="1220" y="216" text-anchor="middle">ship</text>

  <g transform="translate(82 404)"><rect width="324" height="104" rx="12" fill="#fff7f3" stroke="#d8b2a8" stroke-width="2"/><text class="card-title" x="162" y="41" text-anchor="middle">Guard Rejection</text><text class="detail" x="162" y="70" text-anchor="middle">pay fails when total_cents &lt;= 0</text><text class="detail" x="162" y="92" text-anchor="middle">state remains submitted</text></g>
  <path class="cancel" d="M 485 294 L 485 456 L 406 456"/><text class="label" x="446" y="438" text-anchor="middle">guard error</text>

  <g transform="translate(532 606)"><rect width="416" height="104" rx="12" fill="#f5d3df" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="208" y="41" text-anchor="middle">Cancelled</text><text class="detail" x="208" y="70" text-anchor="middle">final state from draft, submitted, or paid</text><text class="detail" x="208" y="92" text-anchor="middle">later commands return 409 Conflict</text></g>
  <path class="cancel" d="M 185 294 L 185 370 L 48 370 L 48 658 L 532 658"/><text class="label" x="254" y="639" text-anchor="middle">cancel</text>
  <path class="cancel" d="M 540 294 L 540 590 L 600 590 L 600 606"/><text class="label" x="572" y="573" text-anchor="middle">cancel</text>
  <path class="cancel" d="M 795 294 L 795 606"/><text class="label" x="836" y="453" text-anchor="middle">cancel</text>

  <rect x="288" y="752" width="904" height="38" rx="11" fill="#edf1e9" stroke="#5d8a62"/><text class="subtitle" x="740" y="777" text-anchor="middle">The README scenario is intentionally in-memory: it teaches lifecycle safety, not persistence.</text>
</svg>
SVG

cat > "$out_dir/order-lifecycle-state-api-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1480" height="840" viewBox="0 0 1480 840" role="img" aria-labelledby="title desc">
  <title id="title">Order Lifecycle State API Architecture</title>
  <desc id="desc">Architecture diagram for the Gin order lifecycle state API showing client routes, handlers, state machine, guard, and error mapping boundaries.</desc>
  <defs>
    <marker id="arch-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <marker id="arch-success" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#4f8b63"/></marker>
    <marker id="arch-error" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #243845; }
      .subtitle, .detail, .label { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .band-label, .card-title { font-family: 'Architects Daughter'; fill: #243845; }
      .band-label { font-size: 21px; }
      .card-title { font-size: 23px; }
      .detail { font-size: 14px; }
      .label { font-size: 13px; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 16; }
      .main { fill: none; stroke: #47616f; stroke-width: 3; marker-end: url(#arch-main); }
      .success { fill: none; stroke: #4f8b63; stroke-width: 3; marker-end: url(#arch-success); }
      .error { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#arch-error); }
    </style>
  </defs>
  <rect width="1480" height="840" fill="#fbfaf6"/>
  <text class="title" x="740" y="58" text-anchor="middle">Order Lifecycle State API Architecture</text>
  <text class="subtitle" x="740" y="92" text-anchor="middle">Gin owns HTTP routing; bluetape-go/state owns lifecycle legality and concurrent transition safety.</text>

  <rect class="band" x="58" y="140" width="1364" height="146"/><text class="band-label" x="94" y="178">Client boundary</text>
  <g transform="translate(96 190)"><rect width="242" height="74" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="121" y="32" text-anchor="middle">HTTP Client</text><text class="detail" x="121" y="56" text-anchor="middle">curl, browser, test</text></g>
  <g transform="translate(1128 184)"><rect width="246" height="86" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="123" y="35" text-anchor="middle">JSON Response</text><text class="detail" x="123" y="60" text-anchor="middle">snapshot or error code</text></g>

  <rect class="band" x="58" y="326" width="1364" height="196"/><text class="band-label" x="94" y="364">Gin API boundary</text>
  <g transform="translate(254 394)"><rect width="242" height="90" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="121" y="35" text-anchor="middle">Gin Router</text><text class="detail" x="121" y="62" text-anchor="middle">health, current, command</text></g>
  <g transform="translate(620 384)"><rect width="258" height="110" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="129" y="40" text-anchor="middle">Order Handlers</text><text class="detail" x="129" y="68" text-anchor="middle">parse events</text><text class="detail" x="129" y="90" text-anchor="middle">map state errors</text></g>
  <g transform="translate(974 394)"><rect width="252" height="90" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="card-title" x="126" y="35" text-anchor="middle">Error Mapper</text><text class="detail" x="126" y="62" text-anchor="middle">400, 408, 409, 500</text></g>

  <rect class="band" x="58" y="562" width="1364" height="158"/><text class="band-label" x="94" y="600">Lifecycle boundary</text>
  <g transform="translate(420 622)"><rect width="276" height="82" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="138" y="34" text-anchor="middle">state.Machine</text><text class="detail" x="138" y="60" text-anchor="middle">states, events, final states</text></g>
  <g transform="translate(784 622)"><rect width="276" height="82" rx="12" fill="#f5d3df" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="138" y="34" text-anchor="middle">Payment Guard</text><text class="detail" x="138" y="60" text-anchor="middle">requires positive total</text></g>

  <path class="main" d="M 338 227 L 375 227 L 375 394"/><text class="label" x="422" y="309" text-anchor="middle">GET / POST</text>
  <path class="main" d="M 496 439 L 620 439"/><text class="label" x="558" y="420" text-anchor="middle">route</text>
  <path class="main" d="M 749 494 L 749 622 L 696 622"/><text class="label" x="690" y="558" text-anchor="middle">State / Transition</text>
  <path class="main" d="M 696 663 L 784 663"/><text class="label" x="740" y="644" text-anchor="middle">pay guard</text>
  <path class="error" d="M 878 439 L 974 439"/><text class="label" x="926" y="420" text-anchor="middle">sentinel error</text>
  <path class="success" d="M 878 410 L 1128 226"/><text class="label" x="1015" y="305" text-anchor="middle">snapshot</text>
  <path class="error" d="M 1100 394 L 1160 270"/><text class="label" x="1150" y="347" text-anchor="middle">problem JSON</text>
  <path class="success" d="M 1128 227 L 338 227"/>

  <rect x="190" y="764" width="1100" height="38" rx="11" fill="#edf1e9" stroke="#5d8a62"/><text class="subtitle" x="740" y="789" text-anchor="middle">The example keeps one process-local order for focused lifecycle learning.</text>
</svg>
SVG

cat > "$out_dir/order-lifecycle-state-api-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1480" height="860" viewBox="0 0 1480 860" role="img" aria-labelledby="title desc">
  <title id="title">Order Lifecycle State API Sequence</title>
  <desc id="desc">Sequence diagram showing submit, can-pay guard evaluation, pay transition, final-state rejection, and JSON response mapping.</desc>
  <defs>
    <marker id="seq-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <marker id="seq-success" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#4f8b63"/></marker>
    <marker id="seq-error" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #243845; }
      .subtitle, .label, .small { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .actor { font-family: 'Architects Daughter'; font-size: 20px; fill: #243845; }
      .label { font-size: 13px; }
      .small { font-size: 12px; }
      .lifeline { stroke: #a9bac5; stroke-width: 2; stroke-dasharray: 7 7; }
      .main { fill: none; stroke: #47616f; stroke-width: 2.8; marker-end: url(#seq-main); }
      .success { fill: none; stroke: #4f8b63; stroke-width: 2.8; marker-end: url(#seq-success); }
      .error { fill: none; stroke: #b95f7a; stroke-width: 2.8; marker-end: url(#seq-error); }
    </style>
  </defs>
  <rect width="1480" height="860" fill="#fbfaf6"/>
  <text class="title" x="740" y="58" text-anchor="middle">Order Lifecycle API Sequence</text>
  <text class="subtitle" x="740" y="92" text-anchor="middle">The handler delegates all lifecycle legality to state.Machine, then maps sentinel errors to HTTP status codes.</text>

  <g transform="translate(185 142)"><rect width="180" height="60" rx="8" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="actor" x="90" y="38" text-anchor="middle">Client</text></g>
  <g transform="translate(475 142)"><rect width="190" height="60" rx="8" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="actor" x="95" y="38" text-anchor="middle">Gin Handler</text></g>
  <g transform="translate(780 142)"><rect width="210" height="60" rx="8" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="actor" x="105" y="38" text-anchor="middle">state.Machine</text></g>
  <g transform="translate(1105 142)"><rect width="190" height="60" rx="8" fill="#f5d3df" stroke="#b95f7a" stroke-width="2"/><text class="actor" x="95" y="38" text-anchor="middle">Pay Guard</text></g>
  <path class="lifeline" d="M 275 202 L 275 728"/>
  <path class="lifeline" d="M 570 202 L 570 728"/>
  <path class="lifeline" d="M 885 202 L 885 728"/>
  <path class="lifeline" d="M 1200 202 L 1200 728"/>

  <path class="main" d="M 275 250 L 570 250"/><rect x="347" y="220" width="150" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="422" y="237" text-anchor="middle">POST submit</text>
  <path class="main" d="M 570 294 L 885 294"/><rect x="643" y="264" width="170" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="728" y="281" text-anchor="middle">Transition(submit)</text>
  <path class="success" d="M 885 338 L 570 338"/><rect x="635" y="308" width="185" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="727" y="325" text-anchor="middle">draft -> submitted</text>
  <path class="success" d="M 570 382 L 275 382"/><rect x="343" y="352" width="158" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="422" y="369" text-anchor="middle">200 snapshot</text>

  <path class="main" d="M 275 440 L 570 440"/><rect x="330" y="410" width="184" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="422" y="427" text-anchor="middle">GET pay/can</text>
  <path class="main" d="M 570 484 L 885 484"/><rect x="631" y="454" width="194" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="728" y="471" text-anchor="middle">CanTransition(pay)</text>
  <path class="main" d="M 885 528 L 1200 528"/><rect x="967" y="498" width="150" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="1042" y="515" text-anchor="middle">check total</text>
  <path class="success" d="M 1200 572 L 885 572"/><rect x="961" y="542" width="164" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="1043" y="559" text-anchor="middle">allowed or error</text>
  <path class="success" d="M 885 616 L 570 616"/><rect x="641" y="586" width="174" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="728" y="603" text-anchor="middle">allowed response</text>

  <rect x="130" y="670" width="1220" height="78" rx="14" fill="#fff7f3" stroke="#d8b2a8" stroke-width="2"/>
  <text class="small" x="154" y="700">Conflict path</text>
  <path class="error" d="M 570 724 L 275 724"/><rect x="333" y="694" width="178" height="23" rx="7" fill="#fff7f3"/><text class="label" x="422" y="711" text-anchor="middle">409 error JSON</text>
  <text class="label" x="820" y="712" text-anchor="middle">invalid transition, guard rejection, final state, or concurrent transition conflict</text>

  <rect x="248" y="794" width="984" height="38" rx="11" fill="#edf1e9" stroke="#5d8a62"/><text class="subtitle" x="740" y="819" text-anchor="middle">Tests drive this sequence with allowed, invalid, guard, final-state, bad-request, and concurrent requests.</text>
</svg>
SVG

write_gate "order-lifecycle-state-api-scenario" 7 7 14
write_gate "order-lifecycle-state-api-architecture" 7 8 14
write_gate "order-lifecycle-state-api-sequence" 4 9 9

for name in \
  order-lifecycle-state-api-scenario \
  order-lifecycle-state-api-architecture \
  order-lifecycle-state-api-sequence
do
  validate_svg "$name"
  render_pair "$name"
done
