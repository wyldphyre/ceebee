import {Browser, Events, Window} from "@wailsio/runtime";
import {ReaderService, type BookInfo} from "../bindings/ceebee";
import "./style.css";

const APP_TITLE = "Comic Reader";
const PLACEHOLDER_WIDTH = 600;
const PLACEHOLDER_HEIGHT = 900;

const toolbarEl = document.getElementById("toolbar")!;
const openButton = document.getElementById("open") as HTMLButtonElement;
const layoutButtons = document.querySelectorAll<HTMLButtonElement>("button[data-pages]");
const directionButtons = document.querySelectorAll<HTMLButtonElement>("button[data-direction]");
const scaleButtons = document.querySelectorAll<HTMLButtonElement>("button[data-scale]");
const progressButton = document.getElementById("progress-toggle") as HTMLButtonElement;
const titleEl = document.getElementById("title")!;
const indicatorEl = document.getElementById("indicator")!;
const readerEl = document.getElementById("reader")!;
const spreadEl = document.getElementById("spread")!;
const emptyEl = document.getElementById("empty")!;
const errorEl = document.getElementById("error")!;
const errorMessageEl = document.getElementById("error-message")!;
const progressEl = document.getElementById("progress")!;
const infoButton = document.getElementById("info-toggle") as HTMLButtonElement;
const infoEl = document.getElementById("info")!;
const infoListEl = document.getElementById("info-list")!;
const aboutEl = document.getElementById("about") as HTMLDialogElement;
const progressFillEl = document.getElementById("progress-fill")!;

// A page shown in the current view, with its natural size.
type Slot = { el: HTMLElement; width: number; height: number };

// How pages are sized: fit the whole window, fit its width, or natural size.
type ScaleMode = "window" | "width" | "original";

let book: BookInfo | null = null;
let twoPage = true;
let rtl = false;
let showProgress = true;
let showInfo = false;
let scaleMode: ScaleMode = "window";
let viewIndex = 0;
let slots: Slot[] = [];
let renderToken = 0;

// A scroll position within the current view. Space steps through these.
type Stop = { x: number; y: number };

// The scroll stop last moved to, or -1 once the reader has scrolled by hand.
let stopIndex = 0;

// Decoded images for the current and next views, keyed by URL.
let images = new Map<string, Promise<HTMLImageElement | null>>();

function views(): number[][] {
    if (!book) return [];
    if (twoPage) return book.spreads as number[][];
    return Array.from({length: book.pageCount}, (_, i) => [i]);
}

function pageURL(index: number): string {
    return `/book/${book!.bookId}/page/${index}`;
}

function loadImage(url: string): Promise<HTMLImageElement | null> {
    let promise = images.get(url);
    if (!promise) {
        const img = new Image();
        img.src = url;
        promise = img.decode().then(() => img, () => null);
        images.set(url, promise);
    }
    return promise;
}

function placeholder(): HTMLElement {
    const el = document.createElement("div");
    el.className = "placeholder";
    el.textContent = "Page could not be loaded";
    return el;
}

async function render(atEnd = false) {
    const token = ++renderToken;
    const all = views();
    const pages = all[viewIndex];
    updateToolbar();
    // Save the reading position. Reaching the last view counts as finishing
    // the book, so it opens at the start next time.
    ReaderService.SavePosition(book!.bookId, viewIndex === all.length - 1 ? 0 : pages[0]);

    const loaded = await Promise.all(pages.map((i) => loadImage(pageURL(i))));
    if (token !== renderToken) return;

    slots = loaded.map((img) =>
        img && img.naturalWidth > 0 && img.naturalHeight > 0
            ? {el: img, width: img.naturalWidth, height: img.naturalHeight}
            : {el: placeholder(), width: PLACEHOLDER_WIDTH, height: PLACEHOLDER_HEIGHT});
    spreadEl.replaceChildren(...slots.map((s) => s.el));
    layout();
    // Start at the view's first scroll stop, or its last when going backwards.
    showStop(atEnd ? scrollStops().length - 1 : 0, false);

    // Keep only the current view's images, then preload the next view.
    const keep = new Set(pages.map(pageURL));
    const next = all[viewIndex + 1] ?? [];
    next.forEach((i) => keep.add(pageURL(i)));
    images = new Map([...images].filter(([url]) => keep.has(url)));
    next.forEach((i) => loadImage(pageURL(i)));
}

// Size the pages: either at their natural size, or scaled to fit the reading
// area (or just its width) with all pages at the same height.
function layout() {
    if (scaleMode === "original") {
        for (const s of slots) {
            s.el.style.height = `${s.height}px`;
            s.el.style.width = `${s.width}px`;
        }
        return;
    }
    const totalAspect = slots.reduce((sum, s) => sum + s.width / s.height, 0);
    if (totalAspect === 0) return;
    const fit = () => {
        const byWidth = readerEl.clientWidth / totalAspect;
        const height = Math.floor(scaleMode === "width" ? byWidth : Math.min(readerEl.clientHeight, byWidth));
        for (const s of slots) {
            s.el.style.height = `${height}px`;
            s.el.style.width = `${Math.floor(s.width / s.height * height)}px`;
        }
    };
    // Fitting to width can add a vertical scrollbar that narrows the reading
    // area (on platforms without overlay scrollbars), so fit again if it does.
    const width = readerEl.clientWidth;
    fit();
    if (readerEl.clientWidth !== width) fit();
}

function updateToolbar() {
    layoutButtons.forEach((b) => {
        b.disabled = !book;
        b.setAttribute("aria-pressed", String((b.dataset.pages === "2") === twoPage));
    });
    directionButtons.forEach((b) => {
        b.disabled = !book;
        b.setAttribute("aria-pressed", String((b.dataset.direction === "rtl") === rtl));
    });
    progressButton.setAttribute("aria-pressed", String(showProgress));
    scaleButtons.forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.scale === scaleMode)));
    spreadEl.classList.toggle("rtl", rtl);
    progressEl.classList.toggle("rtl", rtl);
    progressEl.hidden = !book || !showProgress;
    const hasInfo = (book?.metadata?.length ?? 0) > 0;
    infoButton.disabled = !hasInfo;
    infoButton.setAttribute("aria-pressed", String(hasInfo && showInfo));
    infoEl.hidden = !(hasInfo && showInfo);
    syncInfoMenu();
    if (!book) {
        indicatorEl.textContent = "";
        return;
    }
    const pages = views()[viewIndex];
    const first = Math.min(...pages) + 1;
    const last = Math.max(...pages) + 1;
    indicatorEl.textContent = `${first === last ? first : `${first}–${last}`} / ${book.pageCount}`;
    progressFillEl.style.width = `${(last / book.pageCount) * 100}%`;
}

// Jump straight to a view, such as the first or last.
function goTo(index: number) {
    if (!book || index === viewIndex || index < 0 || index >= views().length) return;
    viewIndex = index;
    render();
}

function go(delta: number, atEnd = false) {
    if (!book) return;
    const next = viewIndex + delta;
    if (next < 0 || next >= views().length) return;
    viewIndex = next;
    render(atEnd);
}

// Move through the current view's scroll stops, turning the page at either end.
function step(delta: number) {
    if (!book) return;
    const stops = scrollStops();
    const current = stopIndex >= 0 && stopIndex < stops.length ? stopIndex : nearestStop(stops);
    const next = current + delta;
    if (next < 0) {
        go(-1, true);
    } else if (next >= stops.length) {
        go(1);
    } else {
        showStop(next, true);
    }
}

function showStop(index: number, smooth: boolean) {
    const stops = scrollStops();
    stopIndex = Math.max(0, Math.min(index, stops.length - 1));
    const {x, y} = stops[stopIndex];
    readerEl.scrollTo({left: x, top: y, behavior: smooth ? "smooth" : "instant"});
}

// The scroll positions needed to see every part of the view, in reading order:
// page by page, each top to bottom, and across each row in reading direction.
// If the whole spread fits across the window, its pages are seen side by side,
// so it is stepped through as one block instead.
function scrollStops(): Stop[] {
    const maxX = readerEl.scrollWidth - readerEl.clientWidth;
    const maxY = readerEl.scrollHeight - readerEl.clientHeight;
    const origin = readerEl.getBoundingClientRect();
    const fitsAcross = spreadEl.getBoundingClientRect().width <= readerEl.clientWidth + 1;
    const blocks = fitsAcross ? [spreadEl] : slots.map((s) => s.el);
    const stops: Stop[] = [];
    for (const block of blocks) {
        const r = block.getBoundingClientRect();
        const xs = axisStops(r.left - origin.left + readerEl.scrollLeft, r.width, readerEl.clientWidth, maxX);
        const ys = axisStops(r.top - origin.top + readerEl.scrollTop, r.height, readerEl.clientHeight, maxY);
        if (rtl) xs.reverse();
        for (const y of ys) {
            for (const x of xs) {
                const last = stops[stops.length - 1];
                if (!last || last.x !== x || last.y !== y) stops.push({x, y});
            }
        }
    }
    return stops.length > 0 ? stops : [{x: 0, y: 0}];
}

// Scroll offsets along one axis that show a page spanning start..start+length
// through a viewport of size view: centred if it fits, otherwise evenly
// spaced, overlapping steps from one edge to the other.
function axisStops(start: number, length: number, view: number, max: number): number[] {
    const clamp = (v: number) => Math.round(Math.min(Math.max(v, 0), Math.max(max, 0)));
    if (length <= view + 1) return [clamp(start + (length - view) / 2)];
    const n = Math.ceil(length / view);
    return Array.from({length: n}, (_, k) => clamp(start + k * (length - view) / (n - 1)));
}

function nearestStop(stops: Stop[]): number {
    let best = 0;
    let bestDistance = Infinity;
    stops.forEach((s, i) => {
        const d = Math.hypot(s.x - readerEl.scrollLeft, s.y - readerEl.scrollTop);
        if (d < bestDistance) {
            best = i;
            bestDistance = d;
        }
    });
    return best;
}

function setBook(info: BookInfo) {
    book = info;
    rtl = info.rtl;
    viewIndex = Math.max(0, views().findIndex((v) => v.includes(info.startPage)));
    images = new Map();
    hideError();
    emptyEl.hidden = true;
    titleEl.textContent = info.title;
    infoListEl.replaceChildren(...(info.metadata ?? []).flatMap((field) => {
        const dt = document.createElement("dt");
        dt.textContent = fieldLabel(field.name);
        const dd = document.createElement("dd");
        dd.textContent = field.value;
        return [dt, dd];
    }));
    document.title = `${info.title} — ${APP_TITLE}`;
    Window.SetTitle(document.title);
    render();
}

// Spaces out ComicInfo element names: "StoryArc" becomes "Story Arc".
function fieldLabel(name: string): string {
    return name.replace(/([a-z])([A-Z])/g, "$1 $2");
}

function setInfo(visible: boolean) {
    if (!book?.metadata?.length) return;
    showInfo = visible;
    updateToolbar();
    layout();
    stopIndex = -1;
}

function toggleInfo() {
    setInfo(!showInfo);
}

// Keeps View › Show Info in step with the panel, telling Go only on changes.
let lastInfoState = "";
function syncInfoMenu() {
    const available = (book?.metadata?.length ?? 0) > 0;
    const state = {available, visible: available && showInfo};
    const key = JSON.stringify(state);
    if (key === lastInfoState) return;
    lastInfoState = key;
    Events.Emit("info-state", state);
}

type Action = "next" | "previous" | "right" | "left" | "first" | "last";

// Runs a navigation action, from a key or the Go menu.
function navigate(action: Action) {
    switch (action) {
        case "next": step(1); break;
        case "previous": step(-1); break;
        // Right and left follow the page visually, so they swap in RTL.
        case "right": go(rtl ? -1 : 1); break;
        case "left": go(rtl ? 1 : -1); break;
        case "first": goTo(0); break;
        case "last": goTo(views().length - 1); break;
    }
}

function showAbout() {
    if (!aboutEl.open) aboutEl.showModal();
}

async function open(load: () => Promise<BookInfo | null>) {
    try {
        const info = await load();
        if (info) setBook(info);
    } catch (err) {
        showError(err instanceof Error ? err.message : String(err));
    }
}

function showError(message: string) {
    errorMessageEl.textContent = message;
    errorEl.hidden = false;
}

function hideError() {
    errorEl.hidden = true;
}

openButton.addEventListener("click", () => open(() => ReaderService.OpenDialog()));

layoutButtons.forEach((b) => b.addEventListener("click", () => {
    const wantTwoPage = b.dataset.pages === "2";
    if (wantTwoPage === twoPage) return;
    const page = views()[viewIndex][0];
    twoPage = wantTwoPage;
    viewIndex = views().findIndex((v) => v.includes(page));
    saveViewSettings();
    render();
}));

directionButtons.forEach((b) => b.addEventListener("click", () => {
    rtl = b.dataset.direction === "rtl";
    updateToolbar();
    stopIndex = -1;
}));

scaleButtons.forEach((b) => b.addEventListener("click", () => {
    scaleMode = b.dataset.scale as ScaleMode;
    saveViewSettings();
    updateToolbar();
    layout();
    showStop(0, false);
}));

progressButton.addEventListener("click", () => {
    showProgress = !showProgress;
    saveViewSettings();
    updateToolbar();
    layout();
    stopIndex = -1;
});

document.getElementById("error-close")!.addEventListener("click", hideError);

// WebKit focuses the first button when the window becomes active, which
// shows its focus ring. Undo that unless the user is tabbing through the
// toolbar with the keyboard.
let tabbing = false;
window.addEventListener("keydown", (e) => { if (e.key === "Tab") tabbing = true; }, true);
window.addEventListener("pointerdown", () => tabbing = false, true);
document.addEventListener("focusin", (e) => {
    if (!tabbing && e.target instanceof HTMLButtonElement && !aboutEl.contains(e.target)) {
        e.target.blur();
    }
});

// Don't leave toolbar buttons focused, or Space would press them.
document.querySelectorAll("button").forEach((b) => b.addEventListener("click", () => b.blur()));

window.addEventListener("keydown", (e) => {
    if (aboutEl.open) return;
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "i") {
        e.preventDefault();
        toggleInfo();
        return;
    }
    let action: Action | null = null;
    if (e.key === " ") action = e.shiftKey ? "previous" : "next";
    else if (e.key === "ArrowRight") action = "right";
    else if (e.key === "ArrowLeft") action = "left";
    // Home and End, or ⌘↑ and ⌘↓ on Macs without those keys.
    else if (e.key === "Home" || (e.metaKey && e.key === "ArrowUp")) action = "first";
    else if (e.key === "End" || (e.metaKey && e.key === "ArrowDown")) action = "last";
    if (action) {
        e.preventDefault();
        navigate(action);
    }
});

Events.On("navigate", (e) => {
    if (!aboutEl.open) navigate(e.data as Action);
});
Events.On("show-info", (e) => setInfo(e.data));

window.addEventListener("resize", () => {
    layout();
    stopIndex = -1;
});

// Scrolling by hand means Space continues from the nearest stop.
readerEl.addEventListener("wheel", () => stopIndex = -1, {passive: true});
readerEl.addEventListener("pointerdown", () => stopIndex = -1);

infoButton.addEventListener("click", toggleInfo);
Events.On("show-about", showAbout);
Events.On("show-open-dialog", () => open(() => ReaderService.OpenDialog()));
Events.On("show-toolbar", (e) => {
    toolbarEl.hidden = !e.data;
    layout();
    stopIndex = -1;
});

// Add the shortcuts to the tooltips, with the platform's modifier key.
const mod = navigator.platform.startsWith("Mac") ? "⌘" : "Ctrl+";
openButton.title = `Open a comic (${mod}O)`;
infoButton.title = `Info (${mod}I)`;

const aboutLink = document.getElementById("about-link") as HTMLAnchorElement;
aboutLink.addEventListener("click", (e) => {
    e.preventDefault();
    Browser.OpenURL(aboutLink.href);
});

ReaderService.Version().then((v) => {
    document.getElementById("about-version")!.textContent = `Version ${v}`;
});

Events.On("open-file", (e) => open(() => ReaderService.OpenPath(e.data)));

function saveViewSettings() {
    ReaderService.SaveViewSettings({twoPage, scaleMode, showProgress});
}

// Restore the saved settings before opening any startup file.
updateToolbar();
ReaderService.ViewSettings().then((view) => {
    twoPage = view.twoPage;
    showProgress = view.showProgress;
    if (["window", "width", "original"].includes(view.scaleMode)) {
        scaleMode = view.scaleMode as ScaleMode;
    }
    updateToolbar();
}).finally(() => ReaderService.StartupPath().then((path) => {
    if (path) open(() => ReaderService.OpenPath(path));
}));
