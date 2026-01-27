// static/app.js
(() => {
    // ---------- utils ----------
    const $ = (id) => document.getElementById(id);

    function byteLen(s) {
        return new TextEncoder().encode(s).length;
    }

    function safeTrim(v) {
        return (v ?? "").toString().trim();
    }

    function setText(id, text) {
        const el = $(id);
        if (el) el.textContent = text;
    }

    function buildRawRequestPreview(method, url, body) {
        const lines = [];

        lines.push(`${method} ${url} HTTP/1.1`);
        lines.push(`Content-Type: application/json`);
        lines.push(`Content-Length: ${byteLen(body ?? "")}`);
        lines.push(""); // blank line
        if (body && body.length > 0) lines.push(body);

        return lines.join("\r\n");
    }

    // ---------- query builders ----------
    // Multi-line format (one per line): "key=value" OR "flag" (no equals)
    function buildQueryStringFromMultiline(text) {
        const raw = text || "";
        const lines = raw
            .split("\n")
            .map((s) => s.trim())
            .filter(Boolean);

        if (lines.length === 0) return "";

        const parts = [];
        for (const line of lines) {
            const eq = line.indexOf("=");

            if (eq === -1) {
                // flag
                parts.push(encodeURIComponent(line));
                continue;
            }

            const k = line.slice(0, eq).trim();
            const v = line.slice(eq + 1); // keep everything after the first '='
            if (!k) continue;

            parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
        }

        return `?${parts.join("&")}`;
    }

    // Single-line format: "a=b&flag&x=y"
    // (keep it simple for the "path params" card)
    function buildQueryStringFromSingleLine(text) {
        const raw = safeTrim(text);
        if (!raw) return "";

        // Minimal “cleanup”: split & then encode each token.
        // Supports: "flag" or "k=v" (value may contain '=' if user pre-encoded).
        const tokens = raw.split("&").map((s) => s.trim()).filter(Boolean);
        if (tokens.length === 0) return "";

        const parts = [];
        for (const tok of tokens) {
            const eq = tok.indexOf("=");
            if (eq === -1) {
                parts.push(encodeURIComponent(tok));
            } else {
                const k = tok.slice(0, eq).trim();
                const v = tok.slice(eq + 1); // keep rest
                if (!k) continue;
                parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(v)}`);
            }
        }

        return parts.length ? `?${parts.join("&")}` : "";
    }

    // ---------- path helpers ----------
    function buildEchoWithId() {
        const id = safeTrim($("pathId")?.value);
        if (!id) return "/api/echo";
        return `/api/echo/${encodeURIComponent(id)}`;
    }

    function buildUserPostPath() {
        const userId = safeTrim($("userid")?.value);
        const postId = safeTrim($("postid")?.value);
        if (!userId || !postId) return null;

        return `/api/users/${encodeURIComponent(userId)}/posts/${encodeURIComponent(postId)}`;
    }

    // ---------- fetch helpers ----------
    async function sendApi(method, basePath, queryString, bodyText) {
        const url = `${basePath}${queryString ?? ""}`;

        // Raw preview is presentation-only (browser will add Host/User-Agent/etc.)
        setText("rawRequest", buildRawRequestPreview(method, url, method === "GET" ? "" : (bodyText ?? "")));
        setText("output", `→ ${method} ${url}\n\n`);

        const init = {
            method,
            headers: { "Content-Type": "application/json" },
        };

        // Don't send bodies on GET
        if (method !== "GET") {
            init.body = bodyText ?? "";
        }

        try {
            const res = await fetch(url, init);
            const text = await res.text();

            setText(
                "output",
                `→ ${method} ${url}\n` +
                `Status: ${res.status}\n\n` +
                `Body:\n${text}\n`
            );
        } catch (e) {
            setText("output", `→ ${method} ${url}\nFetch error: ${e}\n`);
        }

        const out = $("output");
        if (out) out.scrollTop = out.scrollHeight;
    }

    async function fetchToUint8Array(url, onProgress) {
        const res = await fetch(url);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);

        const reader = res.body?.getReader();
        if (!reader) {
            const buf = await res.arrayBuffer();
            return new Uint8Array(buf);
        }

        const chunks = [];
        let total = 0;

        while (true) {
            const { value, done } = await reader.read();
            if (done) break;
            if (value) {
                chunks.push(value);
                total += value.byteLength;
                if (onProgress) onProgress(total);
            }
        }

        const out = new Uint8Array(total);
        let offset = 0;
        for (const c of chunks) {
            out.set(c, offset);
            offset += c.byteLength;
        }
        return out;
    }

    // ---------- init ----------
    document.addEventListener("DOMContentLoaded", () => {
        // Cache elements we use often
        const elPath = $("path");
        const elBody = $("body");
        const elQueryGeneral = $("queryGeneral");

        // Default body
        if (elBody) {
            elBody.value = JSON.stringify({ msg: "hello router", ts: Date.now() }, null, 2);
        }

        // --- Main request card buttons (GET/POST/PUT/DELETE/PATCH) ---
        document.querySelectorAll("button[data-method]").forEach((btn) => {
            btn.addEventListener("click", () => {
                const method = btn.dataset.method;
                const base = safeTrim(elPath?.value) || "/api/echo";
                const qs = buildQueryStringFromMultiline(elQueryGeneral?.value || "");
                const bodyText = elBody?.value || "";
                // If JSON, update timestamp (nice demo)
                if (method !== "GET") {
                    try {
                        const obj = JSON.parse(bodyText || "{}");
                        obj.ts = Date.now();
                        elBody.value = JSON.stringify(obj, null, 2);
                    } catch {
                        // ignore if not JSON
                    }
                }
                sendApi(method, base, qs, method === "GET" ? "" : (elBody?.value || ""));
            });
        });

        // --- Static checks ---
        $("rootCheck")?.addEventListener("click", () => sendApi("GET", "/", "", ""));
        $("indexCheck")?.addEventListener("click", () => sendApi("GET", "/index.html", "", ""));
        $("cssCheck")?.addEventListener("click", () => sendApi("GET", "/styles.css", "", ""));
        $("jsCheck")?.addEventListener("click", () => sendApi("GET", "/app.js", "", ""));

        // --- Path helpers ---
        $("useBaseEcho")?.addEventListener("click", () => {
            if (elPath) elPath.value = "/api/echo";
        });

        $("useIdEcho")?.addEventListener("click", () => {
            if (elPath) elPath.value = buildEchoWithId();
        });

        // --- Path param test: /api/users/:userid/posts/:postid ---
        const elQueryUserPost = $("queryUserPost");

        $("pathParamGet")?.addEventListener("click", () => {
            const path = buildUserPostPath();
            if (!path) return alert("Enter user ID and post ID");
            const qs = buildQueryStringFromSingleLine(elQueryUserPost?.value || "");
            sendApi("GET", path, qs, "");
        });

        $("pathParamPost")?.addEventListener("click", () => {
            const path = buildUserPostPath();
            if (!path) return alert("Enter user ID and post ID");
            const qs = buildQueryStringFromSingleLine(elQueryUserPost?.value || "");
            // reuse body box
            const bodyText = elBody?.value || "";
            try {
                const obj = JSON.parse(bodyText || "{}");
                obj.ts = Date.now();
                elBody.value = JSON.stringify(obj, null, 2);
            } catch { }
            sendApi("POST", path, qs, elBody?.value || "");
        });

        // --- Video (direct src) ---
        const video = $("video");
        const videoStatus = $("videoStatus");

        $("loadVideo")?.addEventListener("click", async () => {
            if (!video || !videoStatus) return;

            const url = "/video";
            videoStatus.textContent = `→ Setting video src to ${url}\n`;

            video.pause();
            video.removeAttribute("src");
            video.load();

            video.src = url;
            video.load();

            try {
                await video.play();
                videoStatus.textContent += "Playing.\n";
            } catch (e) {
                videoStatus.textContent += `Play error: ${e}\n`;
            }
        });

        $("unloadVideo")?.addEventListener("click", () => {
            if (!video || !videoStatus) return;

            video.pause();
            video.removeAttribute("src");
            video.load();
            videoStatus.textContent = "Unloaded video.\n";
        });

        // --- Video (chunked endpoint) ---
        const videoChunked = $("videoChunked");
        const videoChunkedStatus = $("videoChunkedStatus");
        let currentObjectURL = null;

        $("loadVideoChunked")?.addEventListener("click", async () => {
            if (!videoChunked || !videoChunkedStatus) return;

            const url = "/video-chunked";

            // reset player + revoke old URL
            videoChunked.pause();
            videoChunked.removeAttribute("src");
            videoChunked.load();
            if (currentObjectURL) {
                URL.revokeObjectURL(currentObjectURL);
                currentObjectURL = null;
            }

            videoChunkedStatus.textContent = `→ GET ${url}\nDownloading (chunked)...\n`;

            try {
                const bytes = await fetchToUint8Array(url, (n) => {
                    videoChunkedStatus.textContent = `→ GET ${url}\nDownloaded: ${n} bytes\n`;
                });

                videoChunkedStatus.textContent += `Assembling blob...\n`;

                const blob = new Blob([bytes], { type: "video/mp4" });
                currentObjectURL = URL.createObjectURL(blob);

                videoChunked.src = currentObjectURL;
                videoChunked.load();

                videoChunkedStatus.textContent += `Playing...\n`;
                await videoChunked.play();

                videoChunkedStatus.textContent += `✅ Playing (${bytes.length} bytes)\n`;
            } catch (e) {
                videoChunkedStatus.textContent += `❌ Error: ${e}\n`;
            }
        });

        $("unloadVideoChunked")?.addEventListener("click", () => {
            if (!videoChunked || !videoChunkedStatus) return;

            videoChunked.pause();
            videoChunked.removeAttribute("src");
            videoChunked.load();

            if (currentObjectURL) {
                URL.revokeObjectURL(currentObjectURL);
                currentObjectURL = null;
            }

            videoChunkedStatus.textContent = "Unloaded.\n";
        });
    });
})();
