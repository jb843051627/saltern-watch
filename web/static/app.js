async function refresh() {
  try {
    const dash = await (await fetch('/api/v1/dashboard')).json();
    const cards = document.getElementById('summary');
    const risk = { none: '无', watch: '关注', alert: '预警' }[dash.rain_risk] || dash.rain_risk;
    cards.innerHTML = [
      card('今日产量', dash.today_tons.toFixed(1) + ' t'),
      card('活跃告警', String(dash.active_alerts)),
      card('降雨风险', risk),
      card('蒸发率', dash.evap_rate_mm.toFixed(1) + ' mm/d'),
      card('待执行输卤', String(dash.pending_jobs)),
    ].join('');

    const tbody = document.querySelector('#ponds tbody');
    tbody.innerHTML = Object.entries(dash.latest_be || {})
      .map(([id, be]) => `<tr><td>${id}</td><td>${be.toFixed(2)}</td></tr>`)
      .join('');

    const alerts = await (await fetch('/api/v1/alerts')).json();
    document.getElementById('alerts').innerHTML = (alerts || [])
      .map(a => `<li>[${a.severity}] ${a.message} ×${a.count}</li>`)
      .join('');
  } catch (e) {
    console.error('refresh failed', e);
  }
}

function card(label, value) {
  return `<div class="card"><div class="label">${label}</div><div class="value">${value}</div></div>`;
}

setInterval(refresh, 15000);
refresh();
