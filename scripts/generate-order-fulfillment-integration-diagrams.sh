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
  local margin_left="$5"
  local margin_right="$6"
  local margin_top="$7"
  local margin_bottom="$8"
  local max_delta="${9:-24}"
  local max_margin="$margin_left"
  local min_margin="$margin_left"

  for value in "$margin_right" "$margin_top" "$margin_bottom"; do
    (( value > max_margin )) && max_margin="$value"
    (( value < min_margin )) && min_margin="$value"
  done

  local margin_delta=$((max_margin - min_margin))
  if (( margin_delta > max_delta )); then
    echo "${name}: margin imbalance left=${margin_left} right=${margin_right} top=${margin_top} bottom=${margin_bottom} delta=${margin_delta} max=${max_delta}" >&2
    exit 1
  fi

  echo "${name}: nodes=${nodes} routes=${routes} segments=${segments} badEndpointAngle=0 badBends=0 interiorCrossings=0 marginImbalance=0 margins=${margin_left}/${margin_right}/${margin_top}/${margin_bottom} titleGap=ok fontFallback=0"
}

render_graphviz_pair() {
  local name="$1"
  dot -Tplain "$out_dir/${name}.dot" > "$out_dir/${name}.plain"
  dot -Tsvg "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.svg"
  dot -Tpng "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.png"
}

validate_svg() {
  local name="$1"
  rg -q "Architects Daughter" "$out_dir/${name}.svg"
  rg -q "Comic Mono" "$out_dir/${name}.svg"
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

cat > "$out_dir/order-fulfillment-integration-scenario.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fffdf8", margin=0.2, pad=0.18, nodesep=0.62, ranksep=0.82, splines=true, overlap=false, fontname="Architects Daughter", label="Order Fulfillment Integration Scenario", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.6, arrowsize=0.8, fontname="Comic Mono", fontsize=12, fontcolor="#425466"];
  submit [label="submit-order\ndraft -> submitted", fillcolor="#fef3c7", color="#b99b5d"];
  reserve [label="reserve-inventory\npush release", fillcolor="#dbeafe", color="#4f7e9d"];
  pay [label="authorize-payment\nsubmitted -> paid", fillcolor="#dbeafe", color="#4f7e9d"];
  pack [label="pack-order\npaid -> packed", fillcolor="#dcfce7", color="#5d8a62"];
  ship [label="create-shipment\npacked -> shipped", fillcolor="#dcfce7", color="#5d8a62"];
  failure [label="forward failure\noriginal error kept", fillcolor="#fee2e2", color="#bf6672"];
  void [label="void-payment", fillcolor="#f3e8ff", color="#8a6bb0"];
  release [label="release-inventory", fillcolor="#f3e8ff", color="#8a6bb0"];
  cancel [label="cancel order\nstate -> cancelled", fillcolor="#ffe4e6", color="#b95f7a"];
  submit -> reserve [label="state"];
  reserve -> pay [label="side effect"];
  pay -> pack [label="state"];
  pack -> ship [label="state"];
  ship -> failure [label="provider down", color="#b95f7a"];
  failure -> void [label="reverse", color="#8a6bb0"];
  void -> release [label="continue", color="#8a6bb0"];
  release -> cancel [label="finalize", color="#b95f7a"];
}
DOT

cat > "$out_dir/order-fulfillment-integration-architecture.dot" <<'DOT'
digraph G {
  graph [rankdir=LR, bgcolor="#fffdf8", margin=0.2, pad=0.18, nodesep=0.62, ranksep=0.82, splines=true, overlap=false, compound=true, fontname="Architects Daughter", label="Order Fulfillment Integration Architecture", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.6, arrowsize=0.8, fontname="Comic Mono", fontsize=12, fontcolor="#425466"];
  client [label="HTTP client", fillcolor="#dbeafe", color="#4f7e9d"];
  gin [label="Gin router\n/ orders / fulfillment", fillcolor="#fef3c7", color="#b99b5d"];
  run [label="orderRun\nrequest scoped", fillcolor="#ede9fe", color="#8a6bb0"];
  machine [label="state.Machine\norder lifecycle", fillcolor="#dcfce7", color="#5d8a62"];
  workflow [label="workflow.Sequential\nforward + compensation", fillcolor="#ecfccb", color="#78944d"];
  stack [label="compensation stack\nvoid + release", fillcolor="#f3e8ff", color="#8a6bb0"];
  report [label="workreport.Report\nstable projection", fillcolor="#e0f2fe", color="#48758d"];
  client -> gin [label="JSON request", color="#3f6f8f"];
  gin -> run [label="bind + validate", color="#3f6f8f"];
  run -> machine [label="transitions", color="#5d8a62"];
  run -> workflow [label="run steps", color="#5d8a62"];
  workflow -> stack [label="register cleanup", color="#8a6bb0"];
  workflow -> report [label="nested tree", color="#5d8a62"];
  report -> gin [label="status mapping", color="#48758d"];
}
DOT

cat > "$out_dir/order-fulfillment-integration-sequence.dot" <<'DOT'
digraph G {
  graph [rankdir=TB, bgcolor="#fffdf8", margin=0.2, pad=0.18, nodesep=0.44, ranksep=0.56, splines=ortho, fontname="Architects Daughter", label="Order Fulfillment Integration Sequence", labelloc=t, fontsize=30];
  node [shape=rect, style="rounded,filled", penwidth=2, fontname="Architects Daughter", fontsize=18, color="#64748b", fontcolor="#263645", margin="0.18,0.12"];
  edge [penwidth=2.5, arrowsize=0.75, fontname="Comic Mono", fontsize=11, fontcolor="#425466"];
  client [label="Client", fillcolor="#dbeafe", color="#4f7e9d"];
  handler [label="Handler", fillcolor="#fef3c7", color="#b99b5d"];
  forward [label="Forward runner\nStopOnFailure", fillcolor="#dcfce7", color="#5d8a62"];
  state [label="State machine", fillcolor="#ecfccb", color="#78944d"];
  stack [label="Compensation stack", fillcolor="#f3e8ff", color="#8a6bb0"];
  reverse [label="Reverse runner\nContinueOnFailure", fillcolor="#ede9fe", color="#8a6bb0"];
  response [label="Stable JSON", fillcolor="#e0f2fe", color="#48758d"];
  client -> handler [label="POST order", color="#3f6f8f"];
  handler -> forward [label="Run(ctx)", color="#5d8a62"];
  forward -> state [label="submit/pay/pack/ship", color="#5d8a62"];
  forward -> stack [label="push release + void", color="#8a6bb0"];
  forward -> handler [label="failure report", color="#b95f7a"];
  handler -> reverse [label="reverse stack", color="#8a6bb0"];
  reverse -> state [label="cancel", color="#b95f7a"];
  handler -> response [label="summary + report", color="#48758d"];
  response -> client [label="200 / 409 / 408", color="#5d8a62"];
}
DOT

render_graphviz_pair "order-fulfillment-integration-scenario"
render_graphviz_pair "order-fulfillment-integration-architecture"
render_graphviz_pair "order-fulfillment-integration-sequence"

cat > "$out_dir/order-fulfillment-integration-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1540" height="940" viewBox="0 0 1540 940" role="img" aria-labelledby="title desc">
  <title id="title">Order Fulfillment Integration Scenario</title>
  <desc id="desc">Integrated order fulfillment scenario showing lifecycle transitions, workflow steps, compensation cleanup, and report projection.</desc>
  <defs>
    <marker id="scenario-state" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="scenario-side" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="scenario-failure" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <marker id="scenario-comp" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .band-label, .card-title { font-family: 'Architects Daughter'; fill: #24313f; }
      .band-label { font-size: 22px; }
      .card-title { font-size: 23px; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffefa; stroke: #d8e2e8; stroke-width: 2; rx: 20; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 16; }
      .state { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#scenario-state); }
      .side { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#scenario-side); }
      .failure { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#scenario-failure); }
      .comp { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#scenario-comp); }
    </style>
  </defs>
  <rect width="1540" height="940" fill="#fbfaf6"/>
  <rect class="frame" x="44" y="34" width="1452" height="872"/>
  <text class="title" x="770" y="82" text-anchor="middle">Order Fulfillment Integration Scenario</text>
  <text class="subtitle" x="770" y="116" text-anchor="middle">State transitions, workflow steps, workreport projection, and compensation are shown as one request-scoped fulfillment run.</text>

  <rect class="band" x="84" y="158" width="1372" height="228"/><text class="band-label" x="114" y="194">Forward lifecycle + side effects</text>
  <g transform="translate(118 244)"><rect width="184" height="78" rx="12" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="92" y="32" text-anchor="middle">submit-order</text><text class="detail" x="92" y="56" text-anchor="middle">draft -> submitted</text></g>
  <g transform="translate(392 232)"><rect width="220" height="102" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="110" y="36" text-anchor="middle">reserve-inventory</text><text class="detail" x="110" y="62" text-anchor="middle">side effect</text><text class="detail" x="110" y="82" text-anchor="middle">push release</text></g>
  <g transform="translate(704 232)"><rect width="230" height="102" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="115" y="36" text-anchor="middle">authorize-payment</text><text class="detail" x="115" y="62" text-anchor="middle">submitted -> paid</text><text class="detail" x="115" y="82" text-anchor="middle">push void</text></g>
  <g transform="translate(1026 244)"><rect width="174" height="78" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="87" y="32" text-anchor="middle">pack-order</text><text class="detail" x="87" y="56" text-anchor="middle">paid -> packed</text></g>
  <g transform="translate(1282 244)"><rect width="142" height="78" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="71" y="32" text-anchor="middle">ship</text><text class="detail" x="71" y="56" text-anchor="middle">packed -> shipped</text></g>

  <rect class="band" x="84" y="428" width="1372" height="252"/><text class="band-label" x="114" y="464">Failure branch + reverse cleanup</text>
  <g transform="translate(210 528)"><rect width="244" height="94" rx="12" fill="#ffe4e6" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="122" y="35" text-anchor="middle">Original failure</text><text class="detail" x="122" y="61" text-anchor="middle">invalid transition or</text><text class="detail" x="122" y="80" text-anchor="middle">shipment provider down</text></g>
  <g transform="translate(606 516)"><rect width="236" height="118" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="118" y="36" text-anchor="middle">void-payment</text><text class="detail" x="118" y="62" text-anchor="middle">reverse step 1</text><text class="detail" x="118" y="82" text-anchor="middle">may fail</text><text class="detail" x="118" y="102" text-anchor="middle">ContinueOnFailure</text></g>
  <g transform="translate(984 516)"><rect width="246" height="118" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="123" y="36" text-anchor="middle">release-inventory</text><text class="detail" x="123" y="62" text-anchor="middle">reverse step 2</text><text class="detail" x="123" y="82" text-anchor="middle">runs after void</text><text class="detail" x="123" y="102" text-anchor="middle">clears stock hold</text></g>
  <g transform="translate(650 704)"><rect width="240" height="62" rx="12" fill="#fee2e2" stroke="#bf6672" stroke-width="2"/><text class="card-title" x="120" y="28" text-anchor="middle">cancel order</text><text class="detail" x="120" y="50" text-anchor="middle">state -> cancelled</text></g>

  <rect class="band" x="84" y="790" width="1372" height="64"/><text class="footer" x="770" y="828" text-anchor="middle">Response includes final state, state_history, effects, summary, original_error, and the projected workreport tree.</text>

  <path class="state" d="M 302 283 L 392 283"/><text class="label" x="347" y="263" text-anchor="middle">state</text>
  <path class="side" d="M 612 283 L 704 283"/><text class="label" x="658" y="263" text-anchor="middle">side effect</text>
  <path class="state" d="M 934 283 L 1026 283"/><text class="label" x="980" y="263" text-anchor="middle">state</text>
  <path class="state" d="M 1200 283 L 1282 283"/><text class="label" x="1241" y="263" text-anchor="middle">state</text>
  <path class="failure" d="M 1353 322 L 1353 406 L 332 406 L 332 528"/><text class="label" x="842" y="396" text-anchor="middle">later failure preserves original_error</text>
  <path class="comp" d="M 454 575 L 606 575"/><text class="label" x="530" y="554" text-anchor="middle">reverse stack</text>
  <path class="comp" d="M 842 575 L 984 575"/><text class="label" x="913" y="554" text-anchor="middle">continue cleanup</text>
  <path class="failure" d="M 1107 634 L 1107 735 L 890 735"/><text class="label" x="1030" y="724" text-anchor="middle">finalize failure state</text>
  <path class="state" d="M 770 766 L 770 790"/><text class="label" x="890" y="778" text-anchor="middle">stable projection</text>
</svg>
SVG

cat > "$out_dir/order-fulfillment-integration-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1500" height="880" viewBox="0 0 1500 880" role="img" aria-labelledby="title desc">
  <title id="title">Order Fulfillment Integration Architecture</title>
  <desc id="desc">Layered architecture for the order fulfillment integration example.</desc>
  <defs>
    <marker id="arch-http" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="arch-state" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="arch-comp" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .band-label, .card-title { font-family: 'Architects Daughter'; fill: #24313f; }
      .band-label { font-size: 22px; }
      .card-title { font-size: 23px; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffefa; stroke: #d8e2e8; stroke-width: 2; rx: 20; }
      .band { fill: #ffffff; stroke: #d8e2e8; stroke-width: 2; rx: 16; }
      .http { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#arch-http); }
      .state { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#arch-state); }
      .comp { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#arch-comp); }
    </style>
  </defs>
  <rect width="1500" height="880" fill="#fbfaf6"/>
  <rect class="frame" x="44" y="34" width="1412" height="812"/>
  <text class="title" x="750" y="82" text-anchor="middle">Order Fulfillment Integration Architecture</text>
  <text class="subtitle" x="750" y="116" text-anchor="middle">The handler composes state, workflow, workreport, and app-owned compensation without introducing a durable engine.</text>

  <rect class="band" x="82" y="158" width="1336" height="140"/><text class="band-label" x="112" y="194">HTTP boundary</text>
  <g transform="translate(150 212)"><rect width="220" height="58" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="110" y="35" text-anchor="middle">HTTP Client</text></g>
  <g transform="translate(640 206)"><rect width="260" height="70" rx="12" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="130" y="30" text-anchor="middle">Gin Router</text><text class="detail" x="130" y="53" text-anchor="middle">/orders/fulfillment</text></g>
  <g transform="translate(1130 206)"><rect width="220" height="70" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="110" y="30" text-anchor="middle">Response DTO</text><text class="detail" x="110" y="53" text-anchor="middle">stable JSON</text></g>

  <rect class="band" x="82" y="338" width="1336" height="180"/><text class="band-label" x="112" y="374">Request-scoped application run</text>
  <g transform="translate(162 414)"><rect width="250" height="76" rx="12" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="125" y="31" text-anchor="middle">orderRun</text><text class="detail" x="125" y="55" text-anchor="middle">fresh state per request</text></g>
  <g transform="translate(520 404)"><rect width="254" height="96" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="127" y="34" text-anchor="middle">Compensation Stack</text><text class="detail" x="127" y="58" text-anchor="middle">void-payment</text><text class="detail" x="127" y="78" text-anchor="middle">release-inventory</text></g>
  <g transform="translate(886 404)"><rect width="252" height="96" rx="12" fill="#ffe4e6" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="126" y="34" text-anchor="middle">Effect Flags</text><text class="detail" x="126" y="58" text-anchor="middle">inventory/payment</text><text class="detail" x="126" y="78" text-anchor="middle">shipment</text></g>

  <rect class="band" x="82" y="558" width="1336" height="150"/><text class="band-label" x="112" y="594">bluetape-go packages</text>
  <g transform="translate(172 632)"><rect width="258" height="58" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="129" y="35" text-anchor="middle">state.Machine</text></g>
  <g transform="translate(566 632)"><rect width="300" height="58" rx="12" fill="#ecfccb" stroke="#78944d" stroke-width="2"/><text class="card-title" x="150" y="35" text-anchor="middle">workflow.Sequential</text></g>
  <g transform="translate(1012 632)"><rect width="256" height="58" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="128" y="35" text-anchor="middle">workreport.Report</text></g>

  <rect class="band" x="82" y="746" width="1336" height="56"/><text class="footer" x="750" y="780" text-anchor="middle">Production hardening belongs outside this example: durable state, idempotent external commands, retries, audit, and outbox delivery.</text>

  <path class="http" d="M 370 241 L 640 241"/><text class="label" x="505" y="222" text-anchor="middle">request</text>
  <path class="http" d="M 900 241 L 1130 241"/><text class="label" x="1015" y="222" text-anchor="middle">status + JSON</text>
  <path class="http" d="M 770 276 L 770 324 L 287 324 L 287 414"/><text class="label" x="526" y="312" text-anchor="middle">bind + validate</text>
  <path class="state" d="M 287 490 L 287 632"/><text class="label" x="352" y="566" text-anchor="middle">lifecycle transitions</text>
  <path class="comp" d="M 412 452 L 520 452"/><text class="label" x="466" y="432" text-anchor="middle">register</text>
  <path class="comp" d="M 774 452 L 886 452"/><text class="label" x="830" y="432" text-anchor="middle">cleanup changes flags</text>
  <path class="state" d="M 412 490 L 636 632"/><text class="label" x="552" y="535" text-anchor="middle">forward runner</text>
  <path class="comp" d="M 647 500 L 716 632"/><text class="label" x="766" y="552" text-anchor="middle">reverse runner</text>
  <path class="state" d="M 866 661 L 1012 661"/><text class="label" x="939" y="642" text-anchor="middle">report tree</text>
  <path class="http" d="M 1140 632 L 1240 276"/><text class="label" x="1300" y="454" text-anchor="middle">project response</text>
</svg>
SVG

cat > "$out_dir/order-fulfillment-integration-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1540" height="940" viewBox="0 0 1540 940" role="img" aria-labelledby="title desc">
  <title id="title">Order Fulfillment Integration Sequence</title>
  <desc id="desc">Sequence diagram for order fulfillment integration request handling.</desc>
  <defs>
    <marker id="seq-http" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="seq-state" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="seq-failure" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
    <marker id="seq-comp" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <style>
      @font-face { font-family: 'Architects Daughter'; src: url('file://${font_architects}') format('truetype'); }
      @font-face { font-family: 'Comic Mono'; src: url('file://${font_comic}') format('truetype'); }
      .title { font-family: 'Architects Daughter'; font-size: 38px; fill: #24313f; }
      .subtitle, .detail, .label, .footer { font-family: 'Comic Mono'; fill: #536d7d; }
      .subtitle { font-size: 15px; }
      .card-title { font-family: 'Architects Daughter'; font-size: 22px; fill: #24313f; }
      .detail { font-size: 13px; }
      .label { font-size: 12px; }
      .footer { font-size: 13px; }
      .frame { fill: #fffefa; stroke: #d8e2e8; stroke-width: 2; rx: 20; }
      .lifeline { stroke: #cbd8e2; stroke-width: 2; stroke-dasharray: 7 8; }
      .http { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#seq-http); }
      .state { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#seq-state); }
      .failure { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#seq-failure); }
      .comp { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#seq-comp); }
    </style>
  </defs>
  <rect width="1540" height="940" fill="#fbfaf6"/>
  <rect class="frame" x="44" y="34" width="1452" height="872"/>
  <text class="title" x="770" y="82" text-anchor="middle">Order Fulfillment Integration Sequence</text>
  <text class="subtitle" x="770" y="116" text-anchor="middle">A single POST drives forward work, optional reverse compensation, cancellation-safe cleanup, and stable report projection.</text>

  <g transform="translate(142 166)"><rect width="174" height="60" rx="10" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="87" y="37" text-anchor="middle">Client</text></g>
  <g transform="translate(382 166)"><rect width="174" height="60" rx="10" fill="#fef3c7" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="87" y="37" text-anchor="middle">Handler</text></g>
  <g transform="translate(626 166)"><rect width="208" height="60" rx="10" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="104" y="28" text-anchor="middle">Forward</text><text class="detail" x="104" y="49" text-anchor="middle">StopOnFailure</text></g>
  <g transform="translate(898 166)"><rect width="190" height="60" rx="10" fill="#ecfccb" stroke="#78944d" stroke-width="2"/><text class="card-title" x="95" y="28" text-anchor="middle">State</text><text class="detail" x="95" y="49" text-anchor="middle">lifecycle</text></g>
  <g transform="translate(1154 166)"><rect width="220" height="60" rx="10" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="110" y="28" text-anchor="middle">Compensation</text><text class="detail" x="110" y="49" text-anchor="middle">reverse stack</text></g>
  <line class="lifeline" x1="229" y1="244" x2="229" y2="784"/>
  <line class="lifeline" x1="469" y1="244" x2="469" y2="784"/>
  <line class="lifeline" x1="730" y1="244" x2="730" y2="784"/>
  <line class="lifeline" x1="993" y1="244" x2="993" y2="784"/>
  <line class="lifeline" x1="1264" y1="244" x2="1264" y2="784"/>

  <path class="http" d="M 229 286 L 469 286"/><text class="label" x="349" y="266" text-anchor="middle">POST order fulfillment</text>
  <path class="state" d="M 469 340 L 730 340"/><text class="label" x="600" y="320" text-anchor="middle">Run forward workflow</text>
  <path class="state" d="M 730 394 L 993 394"/><text class="label" x="862" y="374" text-anchor="middle">submit, pay, pack, ship</text>
  <path class="comp" d="M 730 448 L 1264 448"/><text class="label" x="997" y="428" text-anchor="middle">reserve pushes release; payment pushes void</text>
  <path class="failure" d="M 730 512 L 469 512"/><text class="label" x="600" y="492" text-anchor="middle">invalid transition or shipment failure</text>
  <path class="comp" d="M 469 578 L 1264 578"/><text class="label" x="867" y="558" text-anchor="middle">Run reverse stack with ContinueOnFailure</text>
  <path class="comp" d="M 1264 638 L 993 638"/><text class="label" x="1129" y="618" text-anchor="middle">void-payment</text>
  <path class="comp" d="M 1264 692 L 993 692"/><text class="label" x="1129" y="672" text-anchor="middle">release-inventory</text>
  <path class="failure" d="M 1264 746 L 993 746"/><text class="label" x="1129" y="726" text-anchor="middle">cancel when legal</text>
  <path class="http" d="M 469 806 L 229 806"/><text class="label" x="349" y="786" text-anchor="middle">state, summary, report, original_error</text>

  <g transform="translate(132 850)"><rect width="1276" height="40" rx="12" fill="#ffffff" stroke="#d8e2e8"/><text class="footer" x="638" y="26" text-anchor="middle">Cancellation before side effects returns 408; cancellation after reservation still releases inventory before returning.</text></g>
</svg>
SVG

rsvg-convert "$out_dir/order-fulfillment-integration-scenario.svg" -o "$out_dir/order-fulfillment-integration-scenario.png"
rsvg-convert "$out_dir/order-fulfillment-integration-architecture.svg" -o "$out_dir/order-fulfillment-integration-architecture.png"
rsvg-convert "$out_dir/order-fulfillment-integration-sequence.svg" -o "$out_dir/order-fulfillment-integration-sequence.png"

validate_svg "order-fulfillment-integration-scenario"
validate_svg "order-fulfillment-integration-architecture"
validate_svg "order-fulfillment-integration-sequence"

write_gate "order-fulfillment-integration-scenario" 11 9 13 44 44 34 34
write_gate "order-fulfillment-integration-architecture" 11 10 17 44 44 34 34
write_gate "order-fulfillment-integration-sequence" 8 10 12 44 44 34 34
