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

// Categories suggested in the add-course form (defaults + ones already in use).
let knownCategories = [...DEFAULT_CATEGORIES];

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

  knownCategories = [...new Set([...DEFAULT_CATEGORIES, ...courses.map((c) => c.category).filter(Boolean)])];
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
  closeCategories();
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

// ---- category combobox (styled replacement for <datalist>) -----------------

const catInput = $("category-input");
const catList = $("category-list");
let catActive = -1;

function matchingCategories() {
  const q = catInput.value.trim().toLowerCase();
  if (!q) return knownCategories;
  return knownCategories.filter((c) =>
    c.toLowerCase().includes(q) || (CATEGORY_LABELS[c] || "").toLowerCase().includes(q));
}

function openCategories() {
  const options = matchingCategories();
  if (!options.length) return closeCategories();

  catActive = -1;
  catList.replaceChildren(...options.map((c, i) => {
    const li = el("li", "combo-option");
    li.id = `category-option-${i}`;
    li.setAttribute("role", "option");
    li.dataset.value = c;
    li.appendChild(el("span", null, c));
    if (CATEGORY_LABELS[c]) li.appendChild(el("small", null, CATEGORY_LABELS[c]));
    // mousedown + preventDefault keeps focus in the input (no blur → close race).
    li.addEventListener("mousedown", (e) => {
      e.preventDefault();
      pickCategory(c);
    });
    return li;
  }));
  catList.hidden = false;
  catInput.setAttribute("aria-expanded", "true");

  // Open upwards when there is not enough room below inside the dialog.
  catList.classList.remove("up");
  const room = dialog.getBoundingClientRect().bottom - catInput.getBoundingClientRect().bottom;
  if (catList.offsetHeight + 16 > room) catList.classList.add("up");
}

function closeCategories() {
  catList.hidden = true;
  catActive = -1;
  catInput.setAttribute("aria-expanded", "false");
  catInput.removeAttribute("aria-activedescendant");
}

function pickCategory(value) {
  catInput.value = value;
  closeCategories();
}

function moveActive(step) {
  const items = [...catList.children];
  if (!items.length) return;
  catActive = (catActive + step + items.length) % items.length;
  items.forEach((li, i) => li.classList.toggle("active", i === catActive));
  catInput.setAttribute("aria-activedescendant", items[catActive].id);
  items[catActive].scrollIntoView({ block: "nearest" });
}

catInput.addEventListener("focus", openCategories);
catInput.addEventListener("input", openCategories);
catInput.addEventListener("click", () => { if (catList.hidden) openCategories(); });
catInput.addEventListener("blur", closeCategories);
catInput.addEventListener("keydown", (e) => {
  if (e.key === "ArrowDown" || e.key === "ArrowUp") {
    e.preventDefault();
    if (catList.hidden) openCategories();
    moveActive(e.key === "ArrowDown" ? 1 : -1);
  } else if (e.key === "Enter" && !catList.hidden && catActive >= 0) {
    e.preventDefault(); // pick the option instead of submitting the form
    pickCategory(catList.children[catActive].dataset.value);
  } else if (e.key === "Escape" && !catList.hidden) {
    e.preventDefault(); // close only the list, not the whole dialog
    e.stopPropagation();
    closeCategories();
  }
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
