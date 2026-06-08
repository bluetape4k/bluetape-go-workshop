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

cat > "$out_dir/fulfillment-workflow-runner-scenario.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.65, ranksep=0.8];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  request [label="Fulfillment Request"];
  validate [label="validate-order", fillcolor="#f7e5aa", color="#b99b5d"];
  inventory [label="reserve-inventory", fillcolor="#dbe8d4", color="#5d8a62"];
  payment [label="authorize-payment", fillcolor="#f5d3df", color="#b95f7a"];
  decision [label="shipment-decision", fillcolor="#eadcf5", color="#8a6bb0"];
  ship [label="create-shipment", fillcolor="#edf1e9", color="#5d8a62"];
  skip [label="shipment-skipped", fillcolor="#efe7d2", color="#a78335"];
  failed [label="failed / cancelled report", fillcolor="#fff7f3", color="#d8b2a8"];
  request -> validate [label="sequential"];
  validate -> inventory [label="parallel"];
  validate -> payment [label="parallel"];
  inventory -> decision [label="ok"];
  payment -> decision [label="ok"];
  decision -> ship [label="requires shipment"];
  decision -> skip [label="pickup / digital"];
  inventory -> failed [label="stock fail", color="#b95f7a", fontcolor="#8d4c63"];
  payment -> failed [label="payment fail", color="#b95f7a", fontcolor="#8d4c63"];
}
DOT

cat > "$out_dir/fulfillment-workflow-runner-architecture.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.75, ranksep=0.85];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  client [label="HTTP Client"];
  gin [label="Gin Router", fillcolor="#f7e5aa", color="#b99b5d"];
  handler [label="Fulfillment Handler", fillcolor="#dbe8d4", color="#5d8a62"];
  workflow [label="workflow runners", fillcolor="#eadcf5", color="#8a6bb0"];
  workreport [label="workreport tree", fillcolor="#efe7d2", color="#a78335"];
  response [label="Stable JSON", fillcolor="#edf1e9", color="#5d8a62"];
  client -> gin [label="POST /run"];
  gin -> handler [label="bind JSON"];
  handler -> workflow [label="build per request"];
  workflow -> workreport [label="reports"];
  handler -> response [label="project timestamps out"];
  response -> client [label="200/409/408"];
}
DOT

cat > "$out_dir/fulfillment-workflow-runner-sequence.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fbfaf6", margin=0.2, nodesep=0.7, ranksep=0.9];
  node [shape=box, style="rounded,filled", fontname="Architects Daughter", fontsize=18, color="#48758d", fillcolor="#d7ecf2"];
  edge [fontname="Comic Mono", fontsize=11, color="#47616f", fontcolor="#35505f", penwidth=2];
  client [label="Client"];
  handler [label="Gin Handler", fillcolor="#f7e5aa", color="#b99b5d"];
  sequential [label="Sequential", fillcolor="#eadcf5", color="#8a6bb0"];
  parallel [label="Parallel risk checks", fillcolor="#dbe8d4", color="#5d8a62"];
  conditional [label="Conditional shipment", fillcolor="#efe7d2", color="#a78335"];
  mapper [label="Response mapper", fillcolor="#edf1e9", color="#5d8a62"];
  client -> handler [label="POST /fulfillment/run"];
  handler -> sequential [label="Run(ctx)"];
  sequential -> parallel [label="after validate"];
  parallel -> conditional [label="both branches ok"];
  conditional -> mapper [label="report tree"];
  parallel -> mapper [label="failed or cancelled", color="#b95f7a", fontcolor="#8d4c63"];
  mapper -> client [label="200 / 409 / 408"];
}
DOT

cat > "$out_dir/fulfillment-workflow-runner-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1480" height="820" viewBox="0 0 1480 820" role="img" aria-labelledby="title desc">
  <title id="title">Fulfillment Workflow Scenario</title>
  <desc id="desc">Scenario flow for the fulfillment workflow runner showing sequential validation, parallel risk checks, conditional shipment, failure, and cancellation paths.</desc>
  <defs>
    <marker id="scenario-main" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#47616f"/></marker>
    <marker id="scenario-success" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#4f8b63"/></marker>
    <marker id="scenario-error" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
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
      .error { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#scenario-error); }
    </style>
  </defs>
  <rect width="1480" height="820" fill="#fbfaf6"/>
  <text class="title" x="740" y="58" text-anchor="middle">Fulfillment Workflow Scenario</text>
  <text class="subtitle" x="740" y="92" text-anchor="middle">A request-scoped workflow validates an order, runs risk checks in parallel, then conditionally creates shipment.</text>

  <g transform="translate(76 190)"><rect width="220" height="104" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="110" y="42" text-anchor="middle">Request</text><text class="detail" x="110" y="70" text-anchor="middle">order + scenario flags</text></g>
  <g transform="translate(374 190)"><rect width="230" height="104" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="115" y="42" text-anchor="middle">validate-order</text><text class="detail" x="115" y="70" text-anchor="middle">sequential first step</text></g>

  <g transform="translate(700 120)"><rect width="250" height="104" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="125" y="42" text-anchor="middle">reserve-inventory</text><text class="detail" x="125" y="70" text-anchor="middle">stock branch</text></g>
  <g transform="translate(700 310)"><rect width="250" height="104" rx="12" fill="#f5d3df" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="125" y="42" text-anchor="middle">authorize-payment</text><text class="detail" x="125" y="70" text-anchor="middle">payment branch</text></g>
  <rect x="658" y="112" width="334" height="330" rx="16" fill="none" stroke="#d8e2e8" stroke-width="2"/>
  <text class="label" x="825" y="470" text-anchor="middle">Parallel risk-checks use StopOnFailure</text>

  <g transform="translate(1080 190)"><rect width="230" height="104" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="115" y="42" text-anchor="middle">shipment-decision</text><text class="detail" x="115" y="70" text-anchor="middle">conditional branch</text></g>
  <g transform="translate(960 584)"><rect width="230" height="88" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="115" y="38" text-anchor="middle">create-shipment</text><text class="detail" x="115" y="64" text-anchor="middle">requires shipment</text></g>
  <g transform="translate(1210 584)"><rect width="210" height="88" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="card-title" x="105" y="38" text-anchor="middle">shipment-skipped</text><text class="detail" x="105" y="64" text-anchor="middle">pickup or digital</text></g>

  <g transform="translate(286 584)"><rect width="314" height="96" rx="12" fill="#fff7f3" stroke="#d8b2a8" stroke-width="2"/><text class="card-title" x="157" y="39" text-anchor="middle">Failed or Cancelled</text><text class="detail" x="157" y="67" text-anchor="middle">409 for branch failure</text><text class="detail" x="157" y="87" text-anchor="middle">408 for caller cancellation</text></g>

  <path class="main" d="M 296 242 L 374 242"/><text class="label" x="335" y="222" text-anchor="middle">POST</text>
  <path class="main" d="M 604 242 L 658 242 L 658 172 L 700 172"/><text class="label" x="650" y="154" text-anchor="middle">parallel</text>
  <path class="main" d="M 604 242 L 658 242 L 658 362 L 700 362"/>
  <path class="success" d="M 950 172 L 1010 172 L 1010 242 L 1080 242"/>
  <path class="success" d="M 950 362 L 1010 362 L 1010 242"/>
  <path class="success" d="M 1195 294 L 1195 510 L 1075 510 L 1075 584"/><text class="label" x="1104" y="531" text-anchor="middle">true</text>
  <path class="success" d="M 1230 294 L 1230 510 L 1315 510 L 1315 584"/><text class="label" x="1284" y="531" text-anchor="middle">false</text>
  <path class="error" d="M 825 414 L 825 632 L 600 632"/><text class="label" x="726" y="615" text-anchor="middle">failure cancels sibling</text>
  <path class="error" d="M 489 294 L 489 584"/><text class="label" x="530" y="458" text-anchor="middle">caller cancelled</text>

  <rect x="280" y="746" width="920" height="38" rx="11" fill="#edf1e9" stroke="#5d8a62"/><text class="subtitle" x="740" y="771" text-anchor="middle">The response returns a stable workreport tree without runtime timestamps.</text>
</svg>
SVG

cat > "$out_dir/fulfillment-workflow-runner-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1480" height="840" viewBox="0 0 1480 840" role="img" aria-labelledby="title desc">
  <title id="title">Fulfillment Workflow Runner Architecture</title>
  <desc id="desc">Architecture diagram for the Gin fulfillment workflow runner showing request binding, workflow composition, report projection, and HTTP response mapping.</desc>
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
  <text class="title" x="740" y="58" text-anchor="middle">Fulfillment Workflow Runner Architecture</text>
  <text class="subtitle" x="740" y="92" text-anchor="middle">Gin owns the HTTP boundary; workflow runners own execution order; workreport owns result shape.</text>

  <rect class="band" x="58" y="140" width="1364" height="146"/><text class="band-label" x="94" y="178">HTTP boundary</text>
  <g transform="translate(96 190)"><rect width="242" height="74" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="121" y="32" text-anchor="middle">HTTP Client</text><text class="detail" x="121" y="56" text-anchor="middle">curl, browser, test</text></g>
  <g transform="translate(1110 184)"><rect width="264" height="86" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="132" y="35" text-anchor="middle">Stable JSON</text><text class="detail" x="132" y="60" text-anchor="middle">report tree, no timestamps</text></g>

  <rect class="band" x="58" y="326" width="1364" height="196"/><text class="band-label" x="94" y="364">Gin API boundary</text>
  <g transform="translate(250 394)"><rect width="242" height="90" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="121" y="35" text-anchor="middle">Gin Router</text><text class="detail" x="121" y="62" text-anchor="middle">health, fulfillment/run</text></g>
  <g transform="translate(612 384)"><rect width="276" height="110" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="138" y="40" text-anchor="middle">Fulfillment Handler</text><text class="detail" x="138" y="68" text-anchor="middle">bind JSON</text><text class="detail" x="138" y="90" text-anchor="middle">map report status</text></g>
  <g transform="translate(994 394)"><rect width="232" height="90" rx="12" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="card-title" x="116" y="35" text-anchor="middle">Report Projector</text><text class="detail" x="116" y="62" text-anchor="middle">stable DTO</text></g>

  <rect class="band" x="58" y="562" width="1364" height="158"/><text class="band-label" x="94" y="600">Workflow boundary</text>
  <g transform="translate(220 622)"><rect width="246" height="82" rx="12" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="123" y="34" text-anchor="middle">Sequential</text><text class="detail" x="123" y="60" text-anchor="middle">validate then compose</text></g>
  <g transform="translate(616 622)"><rect width="246" height="82" rx="12" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="123" y="34" text-anchor="middle">Parallel</text><text class="detail" x="123" y="60" text-anchor="middle">risk checks</text></g>
  <g transform="translate(1010 622)"><rect width="246" height="82" rx="12" fill="#f5d3df" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="123" y="34" text-anchor="middle">Conditional</text><text class="detail" x="123" y="60" text-anchor="middle">shipment branch</text></g>

  <path class="main" d="M 338 227 L 371 227 L 371 394"/><text class="label" x="420" y="309" text-anchor="middle">POST /run</text>
  <path class="main" d="M 492 439 L 612 439"/><text class="label" x="552" y="420" text-anchor="middle">bind</text>
  <path class="main" d="M 750 494 L 750 548 L 343 548 L 343 622"/><text class="label" x="525" y="530" text-anchor="middle">build per request</text>
  <path class="main" d="M 466 663 L 616 663"/><text class="label" x="541" y="644" text-anchor="middle">after validate</text>
  <path class="main" d="M 862 663 L 1010 663"/><text class="label" x="936" y="644" text-anchor="middle">all ok</text>
  <path class="success" d="M 1133 622 L 1133 548 L 994 548 L 994 484"/><text class="label" x="1074" y="532" text-anchor="middle">report tree</text>
  <path class="success" d="M 1226 439 L 1300 439 L 1300 270"/><text class="label" x="1336" y="355" text-anchor="middle">200/409/408</text>
  <path class="success" d="M 1110 227 L 338 227"/>
  <path class="error" d="M 739 622 L 739 548 L 994 548"/><text class="label" x="862" y="530" text-anchor="middle">failed/cancelled</text>

  <rect x="238" y="764" width="1004" height="38" rx="11" fill="#edf1e9" stroke="#5d8a62"/><text class="subtitle" x="740" y="789" text-anchor="middle">No durable engine, retry scheduler, database, queue, or mutable workflow context map is introduced.</text>
</svg>
SVG

cat > "$out_dir/fulfillment-workflow-runner-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1480" height="900" viewBox="0 0 1480 900" role="img" aria-labelledby="title desc">
  <title id="title">Fulfillment Workflow Runner Sequence</title>
  <desc id="desc">Sequence diagram showing the Gin handler running a sequential workflow, parallel risk checks, conditional shipment, and response mapping.</desc>
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
  <rect width="1480" height="900" fill="#fbfaf6"/>
  <text class="title" x="740" y="58" text-anchor="middle">Fulfillment Workflow Sequence</text>
  <text class="subtitle" x="740" y="92" text-anchor="middle">A request-scoped runner preserves branch reports while HTTP status mapping stays outside workflow execution.</text>

  <g transform="translate(98 142)"><rect width="170" height="60" rx="8" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="actor" x="85" y="38" text-anchor="middle">Client</text></g>
  <g transform="translate(355 142)"><rect width="190" height="60" rx="8" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="actor" x="95" y="38" text-anchor="middle">Gin Handler</text></g>
  <g transform="translate(640 142)"><rect width="190" height="60" rx="8" fill="#eadcf5" stroke="#8a6bb0" stroke-width="2"/><text class="actor" x="95" y="38" text-anchor="middle">Sequential</text></g>
  <g transform="translate(900 142)"><rect width="190" height="60" rx="8" fill="#dbe8d4" stroke="#5d8a62" stroke-width="2"/><text class="actor" x="95" y="38" text-anchor="middle">Parallel</text></g>
  <g transform="translate(1178 142)"><rect width="210" height="60" rx="8" fill="#efe7d2" stroke="#a78335" stroke-width="2"/><text class="actor" x="105" y="38" text-anchor="middle">Conditional</text></g>
  <path class="lifeline" d="M 183 202 L 183 726"/>
  <path class="lifeline" d="M 450 202 L 450 726"/>
  <path class="lifeline" d="M 735 202 L 735 726"/>
  <path class="lifeline" d="M 995 202 L 995 726"/>
  <path class="lifeline" d="M 1283 202 L 1283 726"/>

  <path class="main" d="M 183 250 L 450 250"/><rect x="242" y="220" width="150" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="317" y="237" text-anchor="middle">POST run</text>
  <path class="main" d="M 450 304 L 735 304"/><rect x="510" y="274" width="166" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="593" y="291" text-anchor="middle">Run(ctx)</text>
  <path class="success" d="M 735 358 L 450 358"/><rect x="504" y="328" width="178" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="593" y="345" text-anchor="middle">validate-order ok</text>
  <path class="main" d="M 735 412 L 995 412"/><rect x="782" y="382" width="166" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="865" y="399" text-anchor="middle">risk-checks</text>
  <path class="success" d="M 995 466 L 735 466"/><rect x="772" y="436" width="184" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="864" y="453" text-anchor="middle">branch reports</text>
  <path class="main" d="M 735 520 L 1283 520"/><rect x="908" y="490" width="202" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="1009" y="507" text-anchor="middle">shipment-decision</text>
  <path class="success" d="M 1283 574 L 735 574"/><rect x="900" y="544" width="218" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="1009" y="561" text-anchor="middle">create or skip report</text>
  <path class="success" d="M 735 628 L 450 628"/><rect x="496" y="598" width="194" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="593" y="615" text-anchor="middle">workreport tree</text>
  <path class="success" d="M 450 682 L 183 682"/><rect x="231" y="652" width="172" height="23" rx="7" fill="#fbfaf6"/><text class="label" x="317" y="669" text-anchor="middle">200 JSON</text>

  <rect x="120" y="738" width="1240" height="84" rx="14" fill="#fff7f3" stroke="#d8b2a8" stroke-width="2"/>
  <text class="small" x="144" y="768">Failure path</text>
  <text class="label" x="688" y="766" text-anchor="middle">risk check failure cancels siblings</text>
  <text class="label" x="1044" y="766" text-anchor="middle">caller cancellation returns cancelled report</text>
  <path class="error" d="M 995 798 L 183 798"/><rect x="504" y="784" width="178" height="23" rx="7" fill="#fff7f3"/><text class="label" x="593" y="801" text-anchor="middle">409 / 408 JSON</text>

  <rect x="258" y="842" width="964" height="38" rx="11" fill="#edf1e9" stroke="#5d8a62"/><text class="subtitle" x="740" y="867" text-anchor="middle">Tests assert completed, failed, skipped, and cancelled report trees.</text>
</svg>
SVG

write_gate "fulfillment-workflow-runner-scenario" 8 9 16
write_gate "fulfillment-workflow-runner-architecture" 6 8 14
write_gate "fulfillment-workflow-runner-sequence" 5 10 10

for name in \
  fulfillment-workflow-runner-scenario \
  fulfillment-workflow-runner-architecture \
  fulfillment-workflow-runner-sequence
do
  validate_svg "$name"
  render_pair "$name"
done
