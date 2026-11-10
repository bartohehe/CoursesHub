// Portal front-end: list courses as cards grouped by category, add / delete
// courses and toggle their completion status.

const STATUS_LABELS = {
  not_started: "Nie rozpoczęty",
  in_progress: "W trakcie",
  completed: "Ukończony",
};

const CATEGORY_LABELS = {
  programming: "Programowanie",
  ml: "Machine Learning",
  data: "Dane",
  inne: "Inne",
};

const DEFAULT_CATEGORIES = ["programming", "ml", "data"];

const $ = (id) => document.getElementById(id);

function el(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text != null) node.textContent = text;
  return node;
}

async function api(url, options) {
  const res = await fetch(url, options);
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  return res.status === 204 ? null : res.json();
}

// ---- rendering -------------------------------------------------------------

function makeStat(value, label) {
  const span = el("span");
  span.appendChild(el("b", null, String(value)));
  span.appendChild(document.createTextNode(" " + label));
  return span;
}

function renderStats(courses) {
  const stats = $("stats");
  stats.replaceChildren(
    makeStat(courses.length, "kursów"),
    makeStat(courses.filter((c) => c.status === "in_progress").length, "w trakcie"),
    makeStat(courses.filter((c) => c.status === "completed").length, "ukończonych"),
  );
  stats.hidden = courses.length === 0;
}

function makeCard(course) {
  const status = course.status || "not_started";
  const card = el("article", "card");

  const top = el("div", "card-top");
  const title = el("h3");
  const link = el("a", "card-link", course.title);
  link.href = course.path;
  title.appendChild(link);
  top.appendChild(title);
  top.appendChild(el("span", `badge ${status}`, STATUS_LABELS[status] || status));
  card.appendChild(top);

  card.appendChild(el("p", "card-desc", course.description || ""));

  const actions = el("div", "card-actions");
  actions.appendChild(el("span", "card-cta", "Otwórz kurs →"));

  const done = status === "completed";
  const toggle = el("button", "btn btn-small", done ? "↺ Cofnij" : "✓ Ukończony");
  toggle.type = "button";
  toggle.title = done ? "Oznacz jako w trakcie" : "Oznacz kurs jako ukończony";
  toggle.addEventListener("click", () =>
    setStatus(course.slug, done ? "in_progress" : "completed", toggle));
  actions.appendChild(toggle);

  const del = el("button", "btn btn-small btn-danger", "Usuń");
  del.type = "button";
  del.title = "Usuń kurs";
  del.addEventListener("click", () => deleteCourse(course, del));
  actions.appendChild(del);

  card.appendChild(actions);
  return card;
}

function groupByCategory(courses) {
  const groups = new Map();
  for (const c of courses) {
    const key = c.category || "inne";
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(c);
  }
  return groups;
}

function render(courses) {
  renderStats(courses);
  $("empty").hidden = courses.length > 0;

  const container = $("categories");
  container.replaceChildren();
  for (const [category, items] of groupByCategory(courses)) {
    const section = el("section", "category");
    section.appendChild(el("h2", "category-title", CATEGORY_LABELS[category] || category));
    const grid = el("div", "grid");
    for (const course of items) grid.appendChild(makeCard(course));
    section.appendChild(grid);
    container.appendChild(section);
  }

  const categories = new Set([...DEFAULT_CATEGORIES, ...courses.map((c) => c.category).filter(Boolean)]);
  $("category-list").replaceChildren(...[...categories].map((c) => {
    const opt = el("option");
    opt.value = c;
    return opt;
  }));
}

async function load() {
  try {
    const courses = await api("/api/courses");
    $("error").hidden = true;
    render(courses);
  } catch (err) {
    console.error("Failed to load courses:", err);
    $("error").hidden = false;
  } finally {
    $("loading").hidden = true;
  }
}

// ---- card actions ----------------------------------------------------------

async function setStatus(slug, status, button) {
  button.disabled = true;
  try {
    await api("/api/progress", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slug, status }),
    });
    await load();
  } catch (err) {
    alert("Nie udało się zmienić statusu: " + err.message);
    button.disabled = false;
  }
}

async function deleteCourse(course, button) {
  if (!confirm(`Usunąć kurs „${course.title}”?\nPlik kursu i zapisany postęp zostaną skasowane.`)) return;
  button.disabled = true;
  try {
    await api(`/api/courses/${encodeURIComponent(course.slug)}`, { method: "DELETE" });
    await load();
  } catch (err) {
    alert("Nie udało się usunąć kursu: " + err.message);
    button.disabled = false;
  }
}

// ---- add-course dialog -----------------------------------------------------

const dialog = $("add-dialog");
const form = $("add-form");
const fields = form.elements;
const formError = $("form-error");
let slugEdited = false;

function slugify(text) {
  return text
    .toLowerCase()
    .replace(/ł/g, "l")
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64)
    .replace(/-+$/, "");
}

function updateSlugPreview() {
  $("slug-preview").textContent = fields.slug.value || "…";
}

function setTitle(value) {
  fields.title.value = value;
  if (!slugEdited) fields.slug.value = slugify(value);
  updateSlugPreview();
}

function openDialog() {
  form.reset();
  slugEdited = false;
  formError.hidden = true;
  updateSlugPreview();
  dialog.showModal();
}

function showFormError(msg) {
  formError.textContent = msg;
  formError.hidden = false;
}

$("add-btn").addEventListener("click", openDialog);
document.querySelectorAll("[data-open-add]").forEach((b) => b.addEventListener("click", openDialog));
$("cancel-btn").addEventListener("click", () => dialog.close());

fields.title.addEventListener("input", () => setTitle(fields.title.value));
fields.slug.addEventListener("input", () => {
  slugEdited = fields.slug.value !== "";
  updateSlugPreview();
});

// Prefill the title from the uploaded file's <title> (or its filename).
fields.file.addEventListener("change", async () => {
  const file = fields.file.files[0];
  if (!file || fields.title.value.trim()) return;
  let title = "";
  try {
    const doc = new DOMParser().parseFromString(await file.text(), "text/html");
    title = doc.title.trim();
  } catch { /* fall back to the filename */ }
  setTitle(title || file.name.replace(/\.html?$/i, ""));
});

form.addEventListener("submit", async (e) => {
  e.preventDefault();
  formError.hidden = true;

  if (!fields.file.files.length) return showFormError("Wybierz plik HTML kursu.");
  if (!fields.title.value.trim()) return showFormError("Podaj tytuł kursu.");
  if (!fields.slug.checkValidity()) {
    return showFormError("Slug: tylko małe litery, cyfry i myślniki (np. web-scraping).");
  }

  const submit = $("submit-btn");
  submit.disabled = true;
  submit.textContent = "Wysyłanie…";
  try {
    await api("/api/courses", { method: "POST", body: new FormData(form) });
    dialog.close();
    await load();
  } catch (err) {
    showFormError(err.message);
  } finally {
    submit.disabled = false;
    submit.textContent = "Dodaj kurs";
  }
});

load();
