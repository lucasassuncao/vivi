// The landing page's backdrop: masked fields, like the detail pane's, and path
// and tree glyphs that drift up in three layers, shifted by scroll and pointer.
(function () {
  var canvas = document.getElementById("backdrop");
  if (!canvas || !canvas.getContext) return;
  var ctx = canvas.getContext("2d");
  var still = matchMedia("(prefers-reduced-motion: reduce)");
  var glyphs = ["├", "└", "─", "│", "▸", "kv/", "v2", "{}", "•••", "*", "/", "~", "y/n"];
  var depths = [0.25, 0.55, 1];
  var items = [], w = 0, h = 0, dpr = 1;
  var px = 0, py = 0, tx = 0, ty = 0, last = 0;

  function colours() {
    var css = getComputedStyle(document.documentElement);
    return { accent: css.getPropertyValue("--accent").trim(), line: css.getPropertyValue("--dim").trim() };
  }
  var ink = colours();

  function seed() {
    items = [];
    var n = Math.round(Math.min(70, (w * h) / 22000));
    for (var i = 0; i < n; i++) {
      var d = depths[i % depths.length];
      var field = Math.random() < 0.45;
      items.push({
        x: Math.random() * w,
        y: Math.random() * h,
        d: d,
        field: field,
        fw: (60 + Math.random() * 100) * d,
        dots: field && Math.random() < 0.6 ? 3 + ((Math.random() * 5) | 0) : 0,
        glyph: glyphs[(Math.random() * glyphs.length) | 0],
        size: 10 + 16 * d,
        speed: 4 + 10 * d,
        accent: Math.random() < 0.3
      });
    }
  }

  function resize() {
    dpr = Math.min(devicePixelRatio || 1, 2);
    w = innerWidth; h = innerHeight;
    canvas.width = w * dpr; canvas.height = h * dpr;
    canvas.style.width = w + "px"; canvas.style.height = h + "px";
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    seed();
  }

  // A value's field: a rounded box, filled with the dots of a masked value.
  function field(x, y, fw, fh, dots) {
    var r = fh / 2;
    ctx.beginPath();
    ctx.moveTo(x + r, y);
    ctx.arcTo(x + fw, y, x + fw, y + fh, r);
    ctx.arcTo(x + fw, y + fh, x, y + fh, r);
    ctx.arcTo(x, y + fh, x, y, r);
    ctx.arcTo(x, y, x + fw, y, r);
    ctx.closePath();
    ctx.stroke();
    var step = Math.min(fh * 0.7, (fw - fh) / Math.max(dots, 1));
    for (var i = 0; i < dots; i++) {
      ctx.beginPath();
      ctx.arc(x + r + step * (i + 0.5), y + r, fh * 0.12, 0, Math.PI * 2);
      ctx.fill();
    }
  }

  function draw(dt) {
    ctx.clearRect(0, 0, w, h);
    px += (tx - px) * 0.05; py += (ty - py) * 0.05;
    var scroll = scrollY;
    for (var i = 0; i < items.length; i++) {
      var it = items[i];
      it.y -= it.speed * dt;
      var y = it.y - scroll * it.d * 0.35;
      var span = h + 200;
      y = ((y % span) + span) % span - 100;
      var x = it.x + px * it.d * 18;
      y += py * it.d * 12;
      ctx.globalAlpha = 0.08 + 0.14 * it.d;
      ctx.strokeStyle = ctx.fillStyle = it.accent ? ink.accent : ink.line;
      if (it.field) {
        ctx.lineWidth = 1 + 2 * it.d;
        field(x, y, it.fw, it.fw * 0.3, it.dots);
      } else {
        ctx.font = it.size + "px 'JetBrains Mono', 'JetBrains Mono Full', monospace";
        ctx.fillText(it.glyph, x, y);
      }
    }
    ctx.globalAlpha = 1;
  }

  function frame(now) {
    var dt = last ? Math.min((now - last) / 1000, 0.05) : 0;
    last = now;
    draw(dt);
    if (!still.matches && !document.hidden) requestAnimationFrame(frame);
  }

  function start() {
    last = 0;
    if (still.matches) draw(0); else requestAnimationFrame(frame);
  }

  addEventListener("resize", function () { resize(); if (still.matches) draw(0); });
  addEventListener("pointermove", function (e) {
    tx = e.clientX / w - 0.5; ty = e.clientY / h - 0.5;
  });
  addEventListener("scroll", function () { if (still.matches) draw(0); }, { passive: true });
  document.addEventListener("visibilitychange", function () { if (!document.hidden) start(); });
  still.addEventListener("change", start);
  new MutationObserver(function () { ink = colours(); if (still.matches) draw(0); })
    .observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });

  resize();
  start();
})();
