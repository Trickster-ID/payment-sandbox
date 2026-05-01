import { REPORT_DIR } from './env.js';

function ts() {
  return new Date().toISOString().replace(/[-:T]/g, '').slice(0, 14);
}

function fmtMs(v) {
  if (v === undefined || v === null) return '-';
  return `${v.toFixed(2)} ms`;
}

function fmtRate(v) {
  if (v === undefined || v === null) return '-';
  return `${(v * 100).toFixed(2)}%`;
}

function metricVal(metrics, name, field) {
  const m = metrics[name];
  if (!m || !m.values) return null;
  return m.values[field] !== undefined ? m.values[field] : null;
}

function generateHTML(data, profile) {
  const m = data.metrics || {};
  const th = data.thresholds || {};

  const totalReqs = metricVal(m, 'http_reqs', 'count') || 0;
  const reqRate = metricVal(m, 'http_reqs', 'rate') || 0;
  const failRate = metricVal(m, 'http_req_failed', 'rate') || 0;
  const p95 = metricVal(m, 'http_req_duration', 'p(95)');
  const p99 = metricVal(m, 'http_req_duration', 'p(99)');
  const avgDur = metricVal(m, 'http_req_duration', 'avg');
  const minDur = metricVal(m, 'http_req_duration', 'min');
  const maxDur = metricVal(m, 'http_req_duration', 'max');
  const checksRate = metricVal(m, 'checks', 'rate');
  const checksPasses = metricVal(m, 'checks', 'passes') || 0;
  const checksFails = metricVal(m, 'checks', 'fails') || 0;
  const iterations = metricVal(m, 'iterations', 'count') || 0;
  const vusMax = metricVal(m, 'vus_max', 'max') || 0;

  const allThresholdsPassed = Object.values(th).every((v) => v.ok !== false);
  const statusBadge = allThresholdsPassed
    ? '<span class="badge pass">PASS</span>'
    : '<span class="badge fail">FAIL</span>';

  const thresholdRows = Object.entries(th)
    .map(([name, info]) => {
      const ok = info.ok !== false;
      return `<tr class="${ok ? 'pass-row' : 'fail-row'}">
        <td>${name}</td>
        <td>${ok ? '✔ PASS' : '✘ FAIL'}</td>
      </tr>`;
    })
    .join('');

  const checksData = (data.root_group && data.root_group.checks) || [];
  const checkRows = checksData.map((c) => {
    const total = c.passes + c.fails;
    const rate = total > 0 ? ((c.passes / total) * 100).toFixed(1) : '0.0';
    const ok = c.fails === 0;
    return `<tr class="${ok ? '' : 'fail-row'}">
      <td>${c.name}</td>
      <td>${c.passes}</td>
      <td>${c.fails}</td>
      <td>${rate}%</td>
    </tr>`;
  }).join('');

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>K6 Report — ${profile}</title>
<style>
  *{box-sizing:border-box;margin:0;padding:0}
  body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#f5f7fa;color:#333;font-size:14px}
  header{background:#1a202c;color:#fff;padding:20px 32px}
  header h1{font-size:1.4rem;margin-bottom:4px}
  header .sub{opacity:.7;font-size:.85rem}
  .status{display:inline-block;margin-left:12px;vertical-align:middle}
  main{max-width:1100px;margin:24px auto;padding:0 16px}
  .cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:16px;margin-bottom:24px}
  .card{background:#fff;border-radius:8px;padding:20px;box-shadow:0 1px 4px rgba(0,0,0,.08)}
  .card .label{font-size:.75rem;text-transform:uppercase;letter-spacing:.05em;color:#718096;margin-bottom:6px}
  .card .value{font-size:1.6rem;font-weight:700;color:#2d3748}
  .card .value.red{color:#e53e3e}
  .card .value.green{color:#38a169}
  section{background:#fff;border-radius:8px;box-shadow:0 1px 4px rgba(0,0,0,.08);margin-bottom:24px;overflow:hidden}
  section h2{padding:16px 20px;border-bottom:1px solid #e2e8f0;font-size:1rem;font-weight:600}
  table{width:100%;border-collapse:collapse}
  th{text-align:left;padding:10px 20px;background:#f7fafc;font-size:.8rem;text-transform:uppercase;letter-spacing:.05em;color:#718096;border-bottom:1px solid #e2e8f0}
  td{padding:10px 20px;border-bottom:1px solid #f0f4f8;font-size:.9rem}
  tr:last-child td{border-bottom:none}
  .pass-row td:first-child::before{content:'✔ ';color:#38a169}
  .fail-row{background:#fff5f5}
  .fail-row td:first-child::before{content:'✘ ';color:#e53e3e}
  .badge{display:inline-block;padding:3px 10px;border-radius:12px;font-size:.75rem;font-weight:700;text-transform:uppercase}
  .badge.pass{background:#c6f6d5;color:#276749}
  .badge.fail{background:#fed7d7;color:#9b2c2c}
</style>
</head>
<body>
<header>
  <h1>K6 Performance Report — ${profile} ${statusBadge}</h1>
  <div class="sub">Generated: ${new Date().toISOString()} &nbsp;|&nbsp; Profile: ${profile}</div>
</header>
<main>
  <div class="cards">
    <div class="card"><div class="label">Total Requests</div><div class="value">${totalReqs.toLocaleString()}</div></div>
    <div class="card"><div class="label">Req / sec</div><div class="value">${reqRate.toFixed(2)}</div></div>
    <div class="card"><div class="label">p95 Duration</div><div class="value ${p95 !== null && p95 > 300 ? 'red' : 'green'}">${fmtMs(p95)}</div></div>
    <div class="card"><div class="label">Failure Rate</div><div class="value ${failRate > 0.01 ? 'red' : 'green'}">${fmtRate(failRate)}</div></div>
    <div class="card"><div class="label">Checks Pass</div><div class="value ${checksRate !== null && checksRate < 0.99 ? 'red' : 'green'}">${fmtRate(checksRate)}</div></div>
    <div class="card"><div class="label">Iterations</div><div class="value">${iterations.toLocaleString()}</div></div>
    <div class="card"><div class="label">Max VUs</div><div class="value">${vusMax}</div></div>
  </div>

  <section>
    <h2>Thresholds</h2>
    <table>
      <tr><th>Threshold</th><th>Result</th></tr>
      ${thresholdRows || '<tr><td colspan="2">No thresholds configured</td></tr>'}
    </table>
  </section>

  <section>
    <h2>HTTP Duration</h2>
    <table>
      <tr><th>Metric</th><th>Value</th></tr>
      <tr><td>Min</td><td>${fmtMs(minDur)}</td></tr>
      <tr><td>Avg</td><td>${fmtMs(avgDur)}</td></tr>
      <tr><td>p(95)</td><td>${fmtMs(p95)}</td></tr>
      <tr><td>p(99)</td><td>${fmtMs(p99)}</td></tr>
      <tr><td>Max</td><td>${fmtMs(maxDur)}</td></tr>
    </table>
  </section>

  <section>
    <h2>Checks (${checksPasses} passed / ${checksFails} failed)</h2>
    <table>
      <tr><th>Check</th><th>Passes</th><th>Fails</th><th>Pass Rate</th></tr>
      ${checkRows || '<tr><td colspan="4">No checks recorded</td></tr>'}
    </table>
  </section>
</main>
</body>
</html>`;
}

export function makeHandleSummary(data, profile) {
  const stamp = ts();
  const html = generateHTML(data, profile);
  const json = JSON.stringify(data, null, 2);

  const jsonPath = `${REPORT_DIR}/${profile}_${stamp}.json`;
  const htmlPath = `${REPORT_DIR}/${profile}_${stamp}.html`;

  const passed = Object.values(data.thresholds || {}).every((v) => v.ok !== false);
  const summary = [
    '',
    `=== K6 ${profile.toUpperCase()} COMPLETE ===`,
    `Status : ${passed ? 'PASS' : 'FAIL'}`,
    `HTML   : ${htmlPath}`,
    `JSON   : ${jsonPath}`,
    '',
  ].join('\n');

  return {
    stdout: summary,
    [jsonPath]: json,
    [htmlPath]: html,
  };
}
