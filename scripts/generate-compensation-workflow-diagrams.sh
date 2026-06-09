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

render_graphviz_pair() {
  local name="$1"
  dot -Tplain "$out_dir/${name}.dot" > "$out_dir/${name}.plain"
  dot -Tsvg "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.svg"
  dot -Tpng "$out_dir/${name}.dot" > "$out_dir/${name}-graphviz.png"
  cp "$out_dir/${name}-graphviz.svg" "$out_dir/${name}.svg"
  cp "$out_dir/${name}-graphviz.png" "$out_dir/${name}.png"
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

cat > "$out_dir/compensation-workflow-scenario.dot" <<'DOT'
digraph G {
  graph [
    rankdir=LR,
    bgcolor="#fffdf8",
    margin=0.2,
    pad=0.18,
    nodesep=0.62,
    ranksep=0.82,
    splines=true,
    overlap=false,
    outputorder=edgesfirst,
    fontnames=svg,
    fontname="Architects Daughter",
    label="Compensation Workflow Scenario",
    labelloc=t,
    fontsize=32,
    fontcolor="#24313f"
  ];
  node [
    shape=rect,
    style="rounded,filled",
    penwidth=2,
    fontname="Architects Daughter",
    fontsize=19,
    color="#64748b",
    fontcolor="#263645",
    margin="0.18,0.12"
  ];
  edge [
    penwidth=2.6,
    arrowsize=0.8,
    fontname="Comic Mono",
    fontsize=12,
    fontcolor="#425466"
  ];

  request [label="Fulfillment\nrequest", fillcolor="#fef3c7", color="#b99b5d"];
  reserve [label="reserve-inventory\nregister release", fillcolor="#dbeafe", color="#4f7e9d"];
  authorize [label="authorize-payment\nregister void", fillcolor="#dbeafe", color="#4f7e9d"];
  shipment [label="create-shipment\nmay fail", fillcolor="#fee2e2", color="#bf6672"];
  complete [label="completed\nside effects kept", fillcolor="#dcfce7", color="#5d8a62"];
  original [label="original error\npreserved", fillcolor="#ffe4e6", color="#b95f7a"];
  void [label="void-payment\nreverse step 1", fillcolor="#f3e8ff", color="#8a6bb0"];
  release [label="release-inventory\nreverse step 2", fillcolor="#f3e8ff", color="#8a6bb0"];
  response [label="stable JSON\nforward + compensation", fillcolor="#e0f2fe", color="#48758d"];

  request -> reserve [label="forward", color="#3f6f8f"];
  reserve -> authorize [label="success", color="#3f6f8f"];
  authorize -> shipment [label="success", color="#3f6f8f"];
  shipment -> complete [label="shipment ok", color="#5d8a62", fontcolor="#426d46"];
  shipment -> original [label="shipment failed", color="#b95f7a", fontcolor="#9f4d67"];
  original -> void [label="compensate reverse", color="#8a6bb0", fontcolor="#745996"];
  void -> release [label="continue on failure", color="#8a6bb0", fontcolor="#745996"];
  release -> response [label="report both trees", color="#5d8a62", fontcolor="#426d46"];
  complete -> response [label="200 OK", color="#5d8a62", fontcolor="#426d46"];
}
DOT

cat > "$out_dir/compensation-workflow-architecture.dot" <<'DOT'
digraph G {
  graph [
    rankdir=LR,
    bgcolor="#fffdf8",
    margin=0.2,
    pad=0.18,
    nodesep=0.62,
    ranksep=0.86,
    splines=true,
    overlap=false,
    compound=true,
    outputorder=edgesfirst,
    fontnames=svg,
    fontname="Architects Daughter",
    label="Compensation Workflow Architecture",
    labelloc=t,
    fontsize=32,
    fontcolor="#24313f"
  ];
  node [
    shape=rect,
    style="rounded,filled",
    penwidth=2,
    fontname="Architects Daughter",
    fontsize=18,
    color="#64748b",
    fontcolor="#263645",
    margin="0.18,0.12"
  ];
  edge [
    penwidth=2.6,
    arrowsize=0.8,
    fontname="Comic Mono",
    fontsize=12,
    fontcolor="#425466"
  ];

  subgraph cluster_http {
    label="HTTP boundary";
    color="#d8e2e8";
    style="rounded";
    fontname="Architects Daughter";
    fontsize=20;
    client [label="HTTP Client", fillcolor="#dbeafe", color="#4f7e9d"];
    gin [label="Gin Router\n/compensation/fulfillment", fillcolor="#fef3c7", color="#b99b5d"];
    mapper [label="Status + JSON\nmapper", fillcolor="#e0f2fe", color="#48758d"];
  }

  subgraph cluster_app {
    label="Application run object";
    color="#d8e2e8";
    style="rounded";
    fontname="Architects Daughter";
    fontsize=20;
    handler [label="Request Handler\nnew run per request", fillcolor="#ede9fe", color="#8a6bb0"];
    stack [label="Compensation Stack\nrelease + void", fillcolor="#f3e8ff", color="#8a6bb0"];
    effects [label="Side-effect Flags\ninventory/payment/shipment", fillcolor="#ffe4e6", color="#b95f7a"];
  }

  subgraph cluster_btg {
    label="bluetape-go boundary";
    color="#d8e2e8";
    style="rounded";
    fontname="Architects Daughter";
    fontsize=20;
    workflow [label="workflow.Sequential\nStopOnFailure / ContinueOnFailure", fillcolor="#dcfce7", color="#5d8a62"];
    report [label="workreport.Report\nnested execution tree", fillcolor="#ecfccb", color="#78944d"];
  }

  client -> gin [label="request", color="#3f6f8f"];
  gin -> handler [label="bind + validate", color="#3f6f8f"];
  handler -> workflow [label="forward runner", color="#5d8a62"];
  workflow -> stack [label="register successful steps", color="#8a6bb0"];
  workflow -> effects [label="mutate request-scoped flags", color="#b95f7a"];
  workflow -> report [label="forward report", color="#5d8a62"];
  handler -> workflow [label="reverse runner on failure", color="#8a6bb0", constraint=false];
  stack -> workflow [label="reverse order", color="#8a6bb0"];
  report -> mapper [label="original error + children", color="#5d8a62"];
  mapper -> client [label="200 / 409 / 408 / 400", color="#5d8a62"];
}
DOT

cat > "$out_dir/compensation-workflow-sequence.dot" <<'DOT'
digraph G {
  graph [
    rankdir=TB,
    bgcolor="#fffdf8",
    margin=0.2,
    pad=0.18,
    nodesep=0.42,
    ranksep=0.58,
    splines=ortho,
    outputorder=edgesfirst,
    fontnames=svg,
    fontname="Architects Daughter",
    label="Compensation Workflow Sequence",
    labelloc=t,
    fontsize=32,
    fontcolor="#24313f"
  ];
  node [
    shape=rect,
    style="rounded,filled",
    penwidth=2,
    fontname="Architects Daughter",
    fontsize=18,
    color="#64748b",
    fontcolor="#263645",
    margin="0.18,0.12"
  ];
  edge [
    penwidth=2.5,
    arrowsize=0.75,
    fontname="Comic Mono",
    fontsize=11,
    fontcolor="#425466"
  ];

  client [label="Client", fillcolor="#dbeafe", color="#4f7e9d"];
  handler [label="Handler", fillcolor="#fef3c7", color="#b99b5d"];
  forward [label="Forward Sequential\nStopOnFailure", fillcolor="#dcfce7", color="#5d8a62"];
  reserve [label="reserve-inventory", fillcolor="#e0f2fe", color="#48758d"];
  authorize [label="authorize-payment", fillcolor="#e0f2fe", color="#48758d"];
  shipment [label="create-shipment", fillcolor="#ffe4e6", color="#b95f7a"];
  stack [label="Compensation Stack", fillcolor="#f3e8ff", color="#8a6bb0"];
  compensation [label="Compensation Sequential\nContinueOnFailure", fillcolor="#f3e8ff", color="#8a6bb0"];
  void [label="void-payment", fillcolor="#ede9fe", color="#8a6bb0"];
  release [label="release-inventory", fillcolor="#ede9fe", color="#8a6bb0"];
  mapper [label="Response Mapper\noriginal error kept", fillcolor="#ecfccb", color="#78944d"];

  client -> handler [label="POST order", color="#3f6f8f"];
  handler -> forward [label="Run(ctx)", color="#5d8a62"];
  forward -> reserve [label="reserve", color="#5d8a62"];
  reserve -> stack [label="push release", color="#8a6bb0"];
  forward -> authorize [label="authorize", color="#5d8a62"];
  authorize -> stack [label="push void", color="#8a6bb0"];
  forward -> shipment [label="ship", color="#5d8a62"];
  shipment -> handler [label="failed report", color="#b95f7a", fontcolor="#9f4d67"];
  handler -> compensation [label="reverse stack", color="#8a6bb0"];
  compensation -> void [label="void", color="#8a6bb0"];
  compensation -> release [label="release", color="#8a6bb0"];
  compensation -> mapper [label="compensation report", color="#8a6bb0"];
  forward -> mapper [label="success path", color="#5d8a62"];
  mapper -> client [label="stable JSON", color="#5d8a62"];
}
DOT

render_graphviz_pair "compensation-workflow-scenario"
render_graphviz_pair "compensation-workflow-architecture"
render_graphviz_pair "compensation-workflow-sequence"

cat > "$out_dir/compensation-workflow-scenario.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1520" height="920" viewBox="0 0 1520 920" role="img" aria-labelledby="title desc">
  <title id="title">Compensation Workflow Scenario</title>
  <desc id="desc">Request-scoped fulfillment workflow showing forward steps, original failure preservation, and reverse compensation cleanup.</desc>
  <defs>
    <marker id="scenario-request" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="scenario-success" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
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
      .request { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#scenario-request); }
      .success { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#scenario-success); }
      .failure { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#scenario-failure); }
      .comp { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#scenario-comp); }
    </style>
  </defs>
  <rect width="1520" height="920" fill="#fbfaf6"/>
  <rect class="frame" x="44" y="26" width="1432" height="858"/>
  <text class="title" x="760" y="72" text-anchor="middle">Compensation Workflow Scenario</text>
  <text class="subtitle" x="760" y="105" text-anchor="middle">Forward work records reversible side effects; a later failure triggers reverse cleanup without hiding the original error.</text>

  <rect class="band" x="82" y="142" width="1356" height="190"/><text class="band-label" x="112" y="178">Forward path</text>
  <g transform="translate(112 220)"><rect width="210" height="82" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="105" y="33" text-anchor="middle">Request</text><text class="detail" x="105" y="58" text-anchor="middle">order + scenario flags</text></g>
  <g transform="translate(408 210)"><rect width="236" height="102" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="118" y="36" text-anchor="middle">reserve-inventory</text><text class="detail" x="118" y="62" text-anchor="middle">sets inventory_reserved</text><text class="detail" x="118" y="82" text-anchor="middle">push release-inventory</text></g>
  <g transform="translate(728 210)"><rect width="236" height="102" rx="12" fill="#dbeafe" stroke="#4f7e9d" stroke-width="2"/><text class="card-title" x="118" y="36" text-anchor="middle">authorize-payment</text><text class="detail" x="118" y="62" text-anchor="middle">sets payment_authorized</text><text class="detail" x="118" y="82" text-anchor="middle">push void-payment</text></g>
  <g transform="translate(1048 210)"><rect width="236" height="102" rx="12" fill="#fee2e2" stroke="#bf6672" stroke-width="2"/><text class="card-title" x="118" y="36" text-anchor="middle">create-shipment</text><text class="detail" x="118" y="62" text-anchor="middle">last forward side effect</text><text class="detail" x="118" y="82" text-anchor="middle">may fail</text></g>

  <rect class="band" x="82" y="376" width="1356" height="238"/><text class="band-label" x="112" y="412">Failure + compensation path</text>
  <g transform="translate(184 478)"><rect width="238" height="94" rx="12" fill="#ffe4e6" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="119" y="35" text-anchor="middle">Original error</text><text class="detail" x="119" y="61" text-anchor="middle">shipment provider</text><text class="detail" x="119" y="80" text-anchor="middle">stays caller-visible</text></g>
  <g transform="translate(562 466)"><rect width="246" height="118" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="123" y="36" text-anchor="middle">void-payment</text><text class="detail" x="123" y="62" text-anchor="middle">reverse step 1</text><text class="detail" x="123" y="82" text-anchor="middle">may fail independently</text><text class="detail" x="123" y="102" text-anchor="middle">ContinueOnFailure</text></g>
  <g transform="translate(946 466)"><rect width="246" height="118" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="123" y="36" text-anchor="middle">release-inventory</text><text class="detail" x="123" y="62" text-anchor="middle">reverse step 2</text><text class="detail" x="123" y="82" text-anchor="middle">runs even if void fails</text><text class="detail" x="123" y="102" text-anchor="middle">clears reservation</text></g>

  <rect class="band" x="82" y="654" width="1356" height="114"/><text class="band-label" x="112" y="690">Response contract</text>
  <g transform="translate(272 704)"><rect width="270" height="50" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="135" y="33" text-anchor="middle">200 completed</text></g>
  <g transform="translate(622 704)"><rect width="296" height="50" rx="12" fill="#e0f2fe" stroke="#48758d" stroke-width="2"/><text class="card-title" x="148" y="33" text-anchor="middle">stable JSON report</text></g>
  <g transform="translate(998 704)"><rect width="270" height="50" rx="12" fill="#fee2e2" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="135" y="33" text-anchor="middle">409 compensated</text></g>

  <path class="request" d="M 322 261 L 408 261"/><text class="label" x="365" y="241" text-anchor="middle">POST</text>
  <path class="success" d="M 644 261 L 728 261"/><text class="label" x="686" y="241" text-anchor="middle">success</text>
  <path class="success" d="M 964 261 L 1048 261"/><text class="label" x="1006" y="241" text-anchor="middle">success</text>
  <path class="success" d="M 1284 261 L 1368 261 L 1368 790 L 407 790 L 407 754"/><text class="label" x="1390" y="495" text-anchor="middle" transform="rotate(90 1390 495)">all forward steps complete</text>
  <path class="failure" d="M 1166 312 L 1166 360 L 303 360 L 303 478"/><text class="label" x="734" y="350" text-anchor="middle">later forward step fails</text>
  <path class="comp" d="M 422 525 L 562 525"/><text class="label" x="492" y="504" text-anchor="middle">reverse stack top</text>
  <path class="comp" d="M 808 525 L 946 525"/><text class="label" x="877" y="504" text-anchor="middle">continue cleanup</text>
  <path class="comp" d="M 1192 525 C 1274 566 1274 674 998 729"/><text class="label" x="1278" y="626" text-anchor="middle">report cleanup outcome</text>
  <path class="request" d="M 760 654 L 760 704"/><text class="label" x="832" y="684" text-anchor="middle">forward + compensation children</text>

  <rect x="214" y="810" width="1092" height="42" rx="12" fill="#ffffff" stroke="#d8e2e8"/>
  <text class="footer" x="760" y="836" text-anchor="middle">Teaching boundary: request-scoped flags stand in for durable inventory, payment, and shipment commands.</text>
</svg>
SVG

cat > "$out_dir/compensation-workflow-architecture.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1480" height="840" viewBox="0 0 1480 840" role="img" aria-labelledby="title desc">
  <title id="title">Compensation Workflow Architecture</title>
  <desc id="desc">Layered architecture for a Gin compensation workflow example using workflow.Sequential and workreport.Report.</desc>
  <defs>
    <marker id="arch-request" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="arch-success" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
    <marker id="arch-comp" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#8a6bb0"/></marker>
    <marker id="arch-failure" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#b95f7a"/></marker>
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
      .request { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#arch-request); }
      .success { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#arch-success); }
      .comp { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#arch-comp); }
      .failure { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#arch-failure); }
    </style>
  </defs>
  <rect width="1480" height="840" fill="#fbfaf6"/>
  <rect class="frame" x="44" y="24" width="1392" height="790"/>
  <text class="title" x="740" y="70" text-anchor="middle">Compensation Workflow Architecture</text>
  <text class="subtitle" x="740" y="103" text-anchor="middle">Gin owns the HTTP boundary; the run object owns the compensation stack; workflow and workreport own execution shape.</text>

  <rect class="band" x="72" y="142" width="1336" height="134"/><text class="band-label" x="104" y="178">HTTP boundary</text>
  <g transform="translate(112 190)"><rect width="222" height="62" rx="12" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="111" y="28" text-anchor="middle">HTTP Client</text><text class="detail" x="111" y="50" text-anchor="middle">curl, browser, test</text></g>
  <g transform="translate(1062 184)"><rect width="268" height="76" rx="12" fill="#edf1e9" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="134" y="31" text-anchor="middle">Stable JSON</text><text class="detail" x="134" y="55" text-anchor="middle">original_error + report tree</text></g>

  <rect class="band" x="72" y="314" width="1336" height="178"/><text class="band-label" x="104" y="350">Application boundary</text>
  <g transform="translate(224 382)"><rect width="230" height="78" rx="12" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="115" y="31" text-anchor="middle">Gin Router</text><text class="detail" x="115" y="55" text-anchor="middle">health, compensation</text></g>
  <g transform="translate(548 372)"><rect width="252" height="98" rx="12" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="126" y="34" text-anchor="middle">Request Handler</text><text class="detail" x="126" y="58" text-anchor="middle">new run per request</text><text class="detail" x="126" y="78" text-anchor="middle">maps status</text></g>
  <g transform="translate(902 372)"><rect width="254" height="98" rx="12" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="127" y="34" text-anchor="middle">Compensation Stack</text><text class="detail" x="127" y="58" text-anchor="middle">registered by success</text><text class="detail" x="127" y="78" text-anchor="middle">reversed on failure</text></g>

  <rect class="band" x="72" y="532" width="1336" height="146"/><text class="band-label" x="104" y="568">bluetape-go boundary</text>
  <g transform="translate(226 600)"><rect width="314" height="66" rx="12" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="157" y="29" text-anchor="middle">workflow.Sequential</text><text class="detail" x="157" y="51" text-anchor="middle">StopOnFailure / ContinueOnFailure</text></g>
  <g transform="translate(714 600)"><rect width="264" height="66" rx="12" fill="#fee2e2" stroke="#b95f7a" stroke-width="2"/><text class="card-title" x="132" y="29" text-anchor="middle">Side-effect Flags</text><text class="detail" x="132" y="51" text-anchor="middle">inventory, payment, shipment</text></g>
  <g transform="translate(1090 600)"><rect width="246" height="66" rx="12" fill="#ecfccb" stroke="#78944d" stroke-width="2"/><text class="card-title" x="123" y="29" text-anchor="middle">workreport.Report</text><text class="detail" x="123" y="51" text-anchor="middle">nested execution tree</text></g>

  <path class="request" d="M 223 252 L 223 340 L 339 340 L 339 382"/><text class="label" x="280" y="366" text-anchor="middle">POST JSON</text>
  <path class="request" d="M 454 421 L 548 421"/><text class="label" x="501" y="401" text-anchor="middle">bind + validate</text>
  <path class="success" d="M 674 470 L 674 536 L 383 536 L 383 600"/><text class="label" x="528" y="552" text-anchor="middle">build forward runner</text>
  <path class="success" d="M 540 633 L 714 633"/><text class="label" x="627" y="613" text-anchor="middle">successful steps mutate flags</text>
  <path class="comp" d="M 800 421 L 902 421"/><text class="label" x="851" y="401" text-anchor="middle">push cleanup</text>
  <path class="comp" d="M 1029 470 L 1029 520 L 500 520 L 500 600"/><text class="label" x="820" y="506" text-anchor="middle">reverse runner on failure</text>
  <path class="success" d="M 978 633 L 1090 633"/><text class="label" x="1034" y="613" text-anchor="middle">project reports</text>
  <path class="failure" d="M 846 600 L 846 560 L 1213 560 L 1213 600"/><text class="label" x="1030" y="548" text-anchor="middle">original failure remains visible</text>
  <path class="success" d="M 1213 600 L 1213 260"/><text class="label" x="1260" y="430" text-anchor="middle">200 / 409 / 408</text>
  <path class="success" d="M 1062 222 L 334 222"/><text class="label" x="698" y="204" text-anchor="middle">stable response</text>

  <rect x="242" y="730" width="996" height="42" rx="12" fill="#ffffff" stroke="#d8e2e8"/>
  <text class="footer" x="740" y="756" text-anchor="middle">No durable saga engine is introduced; durability and idempotent external commands are production concerns.</text>
</svg>
SVG

cat > "$out_dir/compensation-workflow-sequence.svg" <<SVG
<svg xmlns="http://www.w3.org/2000/svg" width="1540" height="940" viewBox="0 0 1540 940" role="img" aria-labelledby="title desc">
  <title id="title">Compensation Workflow Sequence</title>
  <desc id="desc">Sequence diagram showing request handling, forward sequential execution, compensation stack registration, reverse cleanup, and stable response mapping.</desc>
  <defs>
    <marker id="seq-request" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#3f6f8f"/></marker>
    <marker id="seq-success" viewBox="0 0 8 8" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto"><path d="M 1 1 L 7 4 L 1 7 Z" fill="#5d8a62"/></marker>
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
      .request { fill: none; stroke: #3f6f8f; stroke-width: 3; marker-end: url(#seq-request); }
      .success { fill: none; stroke: #5d8a62; stroke-width: 3; marker-end: url(#seq-success); }
      .failure { fill: none; stroke: #b95f7a; stroke-width: 3; marker-end: url(#seq-failure); }
      .comp { fill: none; stroke: #8a6bb0; stroke-width: 3; marker-end: url(#seq-comp); }
    </style>
  </defs>
  <rect width="1540" height="940" fill="#fbfaf6"/>
  <rect class="frame" x="42" y="24" width="1456" height="878"/>
  <text class="title" x="770" y="70" text-anchor="middle">Compensation Workflow Sequence</text>
  <text class="subtitle" x="770" y="103" text-anchor="middle">The forward runner stops on the first failure, then a reverse runner reports cleanup without replacing the original error.</text>

  <g transform="translate(100 154)"><rect width="180" height="62" rx="10" fill="#d7ecf2" stroke="#48758d" stroke-width="2"/><text class="card-title" x="90" y="38" text-anchor="middle">Client</text></g>
  <g transform="translate(328 154)"><rect width="180" height="62" rx="10" fill="#f7e5aa" stroke="#b99b5d" stroke-width="2"/><text class="card-title" x="90" y="38" text-anchor="middle">Handler</text></g>
  <g transform="translate(570 154)"><rect width="214" height="62" rx="10" fill="#dcfce7" stroke="#5d8a62" stroke-width="2"/><text class="card-title" x="107" y="28" text-anchor="middle">Forward Runner</text><text class="detail" x="107" y="50" text-anchor="middle">StopOnFailure</text></g>
  <g transform="translate(844 154)"><rect width="196" height="62" rx="10" fill="#f3e8ff" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="98" y="28" text-anchor="middle">Stack</text><text class="detail" x="98" y="50" text-anchor="middle">release, void</text></g>
  <g transform="translate(1096 154)"><rect width="218" height="62" rx="10" fill="#ede9fe" stroke="#8a6bb0" stroke-width="2"/><text class="card-title" x="109" y="28" text-anchor="middle">Reverse Runner</text><text class="detail" x="109" y="50" text-anchor="middle">ContinueOnFailure</text></g>

  <line class="lifeline" x1="190" y1="230" x2="190" y2="766"/>
  <line class="lifeline" x1="418" y1="230" x2="418" y2="766"/>
  <line class="lifeline" x1="677" y1="230" x2="677" y2="766"/>
  <line class="lifeline" x1="942" y1="230" x2="942" y2="766"/>
  <line class="lifeline" x1="1205" y1="230" x2="1205" y2="766"/>

  <path class="request" d="M 190 270 L 418 270"/><text class="label" x="304" y="250" text-anchor="middle">POST /compensation/fulfillment</text>
  <path class="success" d="M 418 324 L 677 324"/><text class="label" x="548" y="304" text-anchor="middle">Run forward Sequential(ctx)</text>
  <path class="success" d="M 677 378 L 942 378"/><text class="label" x="810" y="358" text-anchor="middle">reserve-inventory pushes release</text>
  <path class="success" d="M 677 432 L 942 432"/><text class="label" x="810" y="412" text-anchor="middle">authorize-payment pushes void</text>
  <path class="failure" d="M 677 486 L 418 486"/><text class="label" x="548" y="466" text-anchor="middle">create-shipment returns failed report</text>
  <path class="comp" d="M 418 552 L 1205 552"/><text class="label" x="812" y="532" text-anchor="middle">run reverse stack with compensation context</text>
  <path class="comp" d="M 1205 606 L 942 606"/><text class="label" x="1074" y="586" text-anchor="middle">void-payment first</text>
  <path class="comp" d="M 1205 660 L 942 660"/><text class="label" x="1074" y="640" text-anchor="middle">release-inventory still runs</text>
  <path class="success" d="M 1205 724 L 418 724"/><text class="label" x="812" y="704" text-anchor="middle">forward report + compensation report + original_error</text>
  <path class="success" d="M 418 778 L 190 778"/><text class="label" x="304" y="758" text-anchor="middle">200 / 409 / 408 stable JSON</text>

  <g transform="translate(109 820)"><rect width="1322" height="42" rx="12" fill="#ffffff" stroke="#d8e2e8"/><text class="footer" x="661" y="27" text-anchor="middle">Caller cancellation before side effects returns cancelled; cancellation after a side effect still runs registered cleanup.</text></g>
</svg>
SVG

rsvg-convert "$out_dir/compensation-workflow-scenario.svg" -o "$out_dir/compensation-workflow-scenario.png"
rsvg-convert "$out_dir/compensation-workflow-architecture.svg" -o "$out_dir/compensation-workflow-architecture.png"
rsvg-convert "$out_dir/compensation-workflow-sequence.svg" -o "$out_dir/compensation-workflow-sequence.png"

validate_svg "compensation-workflow-scenario"
validate_svg "compensation-workflow-architecture"
validate_svg "compensation-workflow-sequence"

write_gate "compensation-workflow-scenario" 9 9 14
write_gate "compensation-workflow-architecture" 8 10 18
write_gate "compensation-workflow-sequence" 10 14 20
