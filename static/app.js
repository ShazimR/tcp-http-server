async function send(method, overridePath) {
    const path =
        overridePath ??
        document.getElementById("path").value.trim() ??
        "/api/echo";

    const out = document.getElementById("output");
    const bodyEl = document.getElementById("body");

    let body = null;
    if (method !== "GET") {
        try {
            const obj = JSON.parse(bodyEl.value || "{}");
            obj.ts = Date.now();
            bodyEl.value = JSON.stringify(obj, null, 2);
        } catch { }

        body = bodyEl.value;
    }

    out.textContent = `\n→ ${method} ${path}\n`;

    try {
        const res = await fetch(path, {
            method,
            headers: { "Content-Type": "application/json" },
            body,
        });

        const text = await res.text();

        out.textContent =
            `Status: ${res.status}\n` +
            `Body:\n${text}\n`;
    } catch (e) {
        out.textContent = `Fetch error: ${e}\n`;
    }

    out.scrollTop = out.scrollHeight;
}

document.addEventListener("DOMContentLoaded", () => {
    document.querySelectorAll("button[data-method]").forEach((btn) => {
        btn.addEventListener("click", () => send(btn.dataset.method));
    });

    document.getElementById("rootCheck").onclick = () => send("GET", "/");
    document.getElementById("indexCheck").onclick = () =>
        send("GET", "/index.html");
    document.getElementById("cssCheck").onclick = () =>
        send("GET", "/styles.css");
    document.getElementById("jsCheck").onclick = () =>
        send("GET", "/app.js");

    document.getElementById("body").value = JSON.stringify(
        { msg: "hello router", ts: Date.now() },
        null,
        2
    );
});
