/* SyntaxOrigin Multi Tools — masaüstü istemcisi (vanilla JS, harici kütüphane yok). */
(() => {
    "use strict";

    const App = window.go.main.App;
    const RT = window.runtime;

    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => [...document.querySelectorAll(sel)];

    // ---- durum ----
    let kind = "video";
    let formats = null;
    let inputPath = "";
    let outputPath = "";   // otomatik önerilir, kullanıcı "Seç…" ile değiştirir
    let dlMode = "video";
    let dlDir = "";
    let checkedTools = [];

    const convJobs = new Map(); // id -> row
    const dlJobs = new Map();   // id -> row

    // ---- yardımcılar ----
    const docExtOf = {
        pdf_to_image: "png", pdf_to_text: "txt",
        docx_to_pdf: "pdf", pdf_to_docx: "docx",
    };

    const msgKey = (m) => (m && m[0] === "@" ? t(m.slice(1)) : m);

    function targetExt(kind, format) {
        if (kind === "document") return docExtOf[format] || "bin";
        return format;
    }

    function baseName(p) { return (p || "").split(/[\\/]/).pop() || ""; }
    function dirOf(p) { return (p || "").slice(0, Math.max(0, (p || "").length - baseName(p).length)); }
    function stripExt(p) { return baseName(p).replace(/\.[^.]+$/, ""); }

    function suggestOutput() {
        if (!inputPath) return "";
        const ext = targetExt(kind, $("#formatSelect").value);
        return (dirOf(inputPath) + stripExt(inputPath) + "." + ext);
    }

    // ---- çeviri / dil ----
    function applyDynamicI18n() {
        $("#viewTitle").textContent = t("title." + currentView);
        updateCounters();
        const opt = $("#dlQuality");
        if (opt && opt.options) {
            for (let i = 0; i < opt.options.length; i++) {
                if (opt.options[i].value === "best") opt.options[i].textContent = t("q.best");
            }
        }
        const sel = $("#formatSelect");
        if (sel && sel.options.length) resetKindControls();
        if ($("#toolList").children.length) renderToolsList();
        retranslateJobs();
    }

    // ---- görünüm geçişi ----
    let currentView = "convert";

    function showView(name) {
        currentView = name;
        $$(".nav-item").forEach((n) => n.classList.toggle("active", n.dataset.view === name));
        $$(".view").forEach((v) => v.classList.toggle("active", v.id === "view-" + name));
        $("#viewTitle").textContent = t("title." + name);
    }

    $$(".nav-item").forEach((item) => item.addEventListener("click", () => showView(item.dataset.view)));

    initLangSelect();

    // ---- araç sağlık çipleri ----
    async function loadHealth() {
        try {
            checkedTools = await App.Tools();
        } catch (e) {
            checkedTools = [];
        }
        const h = $("#health");
        h.innerHTML = "";
        checkedTools.forEach((tool) => {
            const el = document.createElement("span");
            el.className = "chip " + (tool.found ? "ok" : "missing");
            el.title = (tool.path || tool.name) + (tool.found ? " · " + tool.version : t("chip.missing"));
            el.innerHTML = `<b>${tool.name}</b>`;
            el.addEventListener("click", () => showView("tools"));
            h.appendChild(el);
        });
    }

    async function loadDownloadDir() {
        try {
            dlDir = await App.DownloadDir();
            $("#dlDir").value = dlDir;
        } catch (e) { /* yoksa boş */ }
    }

    // ---- dönüştürme sekmeleri / formatlar ----
    function resetKindControls() {
        const sel = $("#formatSelect");
        const prev = sel.value;
        sel.innerHTML = "";
        (formats[kind] || []).forEach((f) => {
            const o = document.createElement("option");
            o.value = f.value;
            const key = "fmt." + f.value;
            const tr = t(key);
            o.textContent = tr !== key ? tr : f.label;
            sel.appendChild(o);
        });
        if (prev && [...sel.options].some((o) => o.value === prev)) sel.value = prev;

        const isDoc = kind === "document";
        $("#optBitrate").classList.toggle("hidden", isDoc || kind === "image");
        $("#optSize").classList.toggle("hidden", kind !== "image");

        if (outputPath && inputPath) outputPath = suggestOutput();
        refreshOutput();
        enableConvert();
    }

    $$(".seg-btn[data-kind]").forEach((b) => b.addEventListener("click", () => {
        $$(".seg-btn[data-kind]").forEach((x) => x.classList.remove("active"));
        b.classList.add("active");
        kind = b.dataset.kind;
        resetKindControls();
    }));

    // ---- kaynak seçimi ----
    $("#btnPickInput").addEventListener("click", async () => {
        try {
            const p = await App.PickInputFile();
            if (!p) return;
            inputPath = p;
            $("#inputPath").value = p;
            $("#btnPickOutput").disabled = false;
            outputPath = suggestOutput();
            $("#outputPath").value = outputPath;
            enableConvert();
        } catch (e) {
            toast(tpl("toast.pickFile", { msg: e }), true);
        }
    });

    // ---- çıktı seçimi ----
    $("#btnPickOutput").addEventListener("click", async () => {
        try {
            const p = await App.PickOutputFile(inputPath, kind, $("#formatSelect").value);
            if (!p) return;
            outputPath = p;
            $("#outputPath").value = p;
            enableConvert();
        } catch (e) {
            toast(tpl("toast.pickOutput", { msg: e }), true);
        }
    });

    function refreshOutput() {
        $("#outputPath").value = outputPath || "";
    }

    function enableConvert() {
        $("#btnConvert").disabled = !(inputPath && outputPath && baseName(outputPath).includes("."));
    }

    // ---- dönüştürme başlat ----
    $("#btnConvert").addEventListener("click", async () => {
        const format = $("#formatSelect").value;
        const req = {
            input: inputPath,
            kind,
            format,
            bitrate: numOr0("#bitrateInput"),
            width: numOr0("#widthInput"),
            height: numOr0("#heightInput"),
            outputPath,
        };
        $("#btnConvert").disabled = true;
        try {
            const id = await App.Convert(req);
            const label = `${baseName(inputPath)} → ${$("#formatSelect").selectedOptions[0]?.textContent || format}`;
            seedRow(convJobs, $("#convJobs"), id, label, "convert");
        } catch (e) {
            toast(tpl("toast.convertStart", { msg: e }), true);
        }
        enableConvert();
        updateCounters();
    });

    function numOr0(sel) { const n = parseInt($(sel).value, 10); return Number.isFinite(n) && n > 0 ? n : 0; }

    // ---- indirme ----
    $$(".seg-btn[data-mode]").forEach((b) => b.addEventListener("click", () => {
        $$(".seg-btn[data-mode]").forEach((x) => x.classList.remove("active"));
        b.classList.add("active");
        dlMode = b.dataset.mode;
        $("#optQuality").classList.toggle("hidden", dlMode !== "video");
    }));

    $("#btnPickDir").addEventListener("click", async () => {
        try {
            dlDir = await App.PickDownloadDir();
            $("#dlDir").value = dlDir;
        } catch (e) { /* iptal */ }
    });

    $("#dlUrl").addEventListener("input", () => {
        $("#btnDownload").disabled = !$("#dlUrl").value.trim();
    });

    $("#btnDownload").addEventListener("click", async () => {
        const req = {
            url: $("#dlUrl").value.trim(),
            mode: dlMode,
            quality: $("#dlQuality").value,
            outDir: dlDir,
        };
        $("#btnDownload").disabled = true;
        try {
            const id = await App.Download(req);
            seedRow(dlJobs, $("#dlJobs"), id, req.url, "download");
            $("#dlUrl").value = "";
            $("#btnDownload").disabled = true;
        } catch (e) {
            toast(tpl("toast.downloadStart", { msg: e }), true);
            $("#btnDownload").disabled = false;
        }
        updateCounters();
    });

    // ---- işler (ortak) ----
    function seedRow(map, container, id, label, kind_) {
        if (map.has(id)) return;
        const row = document.createElement("div");
        row.className = "job";
        row.innerHTML = `
            <div class="job-head">
                <span class="job-label"></span>
                <span class="job-state running"></span>
            </div>
            <div class="bar"><div class="bar-fill"></div></div>
            <div class="pct">0%</div>
            <div class="job-msg"></div>
            <div class="job-out"></div>`;
        row.querySelector(".job-label").textContent = label;
        const empty = container.querySelector(".empty");
        if (empty) empty.remove();
        container.prepend(row);
        const rec = { el: row, kind: kind_, last: { id, state: "running", progress: 0 } };
        map.set(id, rec);
        applyJobRow(map, rec.last);
        return rec;
    }

    function applyJob(e) {
        if (e.kind === "convert") return applyJobRow(convJobs, e);
        if (e.kind === "download") return applyJobRow(dlJobs, e);
        if (e.kind === "install") return applyInstall(e);
    }

    function applyJobRow(map, e) {
        const rec = map.get(e.id);
        if (!rec) return;
        rec.last = e;
        const { el } = rec;
        const st = el.querySelector(".job-state");
        const fill = el.querySelector(".bar-fill");
        const pct = el.querySelector(".pct");
        const msg = el.querySelector(".job-msg");

        if (e.state === "done") {
            st.textContent = t("state.done");
            st.className = "job-state done";
            fill.classList.add("done");
            fill.style.width = "100%";
            pct.textContent = "100%";
            if (e.message) { msg.textContent = msgKey(e.message); msg.className = "job-msg"; }
            if (e.output) showJobOut(el, e.output);
        } else if (e.state === "error") {
            st.textContent = t("state.error");
            st.className = "job-state error";
            fill.classList.add("error");
            pct.textContent = t("install.errlbl");
            if (e.message) { msg.textContent = msgKey(e.message); msg.className = "job-msg err"; }
        } else {
            st.textContent = t("state.running");
            st.className = "job-state running";
            fill.style.width = (e.progress || 0) + "%";
            pct.textContent = Math.round(e.progress || 0) + "%";
            if (e.message) { msg.textContent = msgKey(e.message); msg.className = "job-msg"; }
        }
        updateCounters();
    }

    function retranslateJobs() {
        convJobs.forEach((rec) => applyJobRow(convJobs, rec.last));
        dlJobs.forEach((rec) => applyJobRow(dlJobs, rec.last));
    }

    function showJobOut(el, path) {
        const out = el.querySelector(".job-out");
        out.innerHTML = "";
        const b = document.createElement("button");
        b.className = "mini-btn";
        b.textContent = t("btn.openFolder");
        b.addEventListener("click", () => App.OpenFolder(path));
        out.appendChild(b);
    }

    function updateCounters() {
        $("#convCounter").textContent = tpl("jobs.count", { n: convJobs.size });
        $("#dlCounter").textContent = tpl("jobs.count", { n: dlJobs.size });
    }

    // ---- kurulum ----
    async function renderTools() {
        checkedTools = await App.Tools();
        renderToolsList();
        await loadHealth();
    }

    function renderToolsList() {
        const list = $("#toolList");
        list.innerHTML = "";
        checkedTools.forEach((tool) => {
            const card = document.createElement("div");
            card.className = "tool-card";
            const status = tool.found ? "ok" : "missing";
            const ver = tool.found && tool.version ? tool.version : "";
            const actions = document.createElement("div");
            actions.className = "tool-actions";
            if (tool.installable) {
                const btn = document.createElement("button");
                btn.className = "btn ghost";
                btn.textContent = tool.found ? t("btn.reinstall") : t("btn.install");
                btn.addEventListener("click", () => installTool(tool.key, tool.name));
                actions.appendChild(btn);
            } else {
                const g = document.createElement("button");
                g.className = "btn ghost";
                g.textContent = t("btn.official");
                g.addEventListener("click", () => App.OpenExternal(tool.guideUrl));
                actions.appendChild(g);
            }
            card.innerHTML = `
                <div class="tool-ico">${iconFor(tool.key)}</div>
                <div class="tool-info">
                    <h4>${tool.name}</h4>
                    <div class="tool-src">${t("tool.src." + tool.key)}</div>
                    <div class="tool-ver">${tool.found ? (tool.path + (ver ? " · " + ver : "")) : t("status.notinstalled")}</div>
                </div>
                <div class="tool-status ${status}">${tool.found ? t("status.installed") : t("status.missing")}</div>`;
            card.appendChild(actions);
            list.appendChild(card);
        });
    }

    function iconFor(key) {
        switch (key) {
            case "ffmpeg": return "🎞";
            case "yt-dlp": return "⬇";
            case "libreoffice": return "📝";
            case "poppler": return "📄";
            default: return "🧰";
        }
    }

    function installTool(key, name) {
        const card = $("#installCard");
        card.classList.remove("hidden");
        $("#installTitle").textContent = tpl("install.titleWith", { name });
        $("#installFill").style.width = "0%";
        $("#installPct").textContent = "%0";
        $("#installMsg").textContent = t("install.starting");
        App.InstallTool(key).catch((e) => toast(tpl("toast.installStart", { msg: e }), true));
    }

    function applyInstall(e) {
        const card = $("#installCard");
        card.classList.remove("hidden");
        $("#installFill").classList.add("install");
        if (e.state === "done") {
            $("#installFill").style.width = "100%";
            $("#installPct").textContent = "%100";
            $("#installMsg").textContent = t("install.done");
            toast(t("install.success"), false);
            renderTools();
        } else if (e.state === "error") {
            $("#installPct").textContent = t("install.errlbl");
            $("#installMsg").textContent = "✕ " + msgKey(e.message || "");
            $("#installFill").classList.add("error");
            toast(tpl("install.failed", { msg: msgKey(e.message || "") }), true);
        } else {
            $("#installFill").style.width = (e.progress || 0) + "%";
            $("#installPct").textContent = "%" + Math.round(e.progress || 0);
            if (e.message) $("#installMsg").textContent = msgKey(e.message);
        }
    }

    // ---- hakkında ----
    async function fillAbout() {
        try {
            const a = await App.About();
            $("#aboutName").textContent = a.name;
            $("#aboutVer").textContent = a.version;
            $("#aboutGo").textContent = a.goVer;
            $("#sideVer").textContent = "v" + a.version;
            $("#btnOpenRepo").addEventListener("click", () => App.OpenExternal(a.repoUrl));
            $$(".gh-avatar, .gh-link").forEach((el) =>
                el.addEventListener("click", (ev) => { ev.preventDefault(); App.OpenExternal(a.orgUrl); }));
        } catch (e) { /* boş */ }
    }

    // ---- toast ----
    function toast(msg, isErr) {
        const el = document.createElement("div");
        el.className = "toast" + (isErr ? " err" : " ok");
        el.textContent = msg;
        $("#toasts").appendChild(el);
        setTimeout(() => el.remove(), 5200);
    }

    // ---- olay akışı ----
    RT.EventsOn("job", applyJob);

    // ---- başlat ----
    (async () => {
        try {
            formats = await App.Formats();
        } catch (e) {
            formats = null;
        }
        resetKindControls();
        await loadHealth();
        await loadDownloadDir();
        fillAbout();
        renderTools();
        updateCounters();
        applyDynamicI18n();
        $("#dlUrl").addEventListener("input", () => {
            $("#btnDownload").disabled = !$("#dlUrl").value.trim();
        });
    })();
})();