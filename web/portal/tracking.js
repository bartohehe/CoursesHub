/*
 * Courses Hub — tracking postępu.
 *
 * Serwer wstrzykuje ten skrypt automatycznie do każdego kursu (przed </body>),
 * więc nie trzeba niczego dopisywać ręcznie:
 *   <script src="/tracking.js" data-course-slug="<slug>"></script>
 *
 * - Wejście do kursu → status "in_progress".
 * - window.markCourseCompleted() → status "completed". Możesz to podpiąć pod
 *   koniec kursu (ostatni krok, zdany quiz) albo oznaczyć kurs w portalu.
 */
(function () {
  "use strict";

  var current = document.currentScript;
  var slug = current && current.getAttribute("data-course-slug");

  function sendProgress(status, courseSlug) {
    courseSlug = courseSlug || slug;
    if (!courseSlug) {
      console.warn("[courses-hub] brak course slug — pomijam tracking (" + status + ")");
      return Promise.resolve();
    }
    return fetch("/api/progress", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slug: courseSlug, status: status }),
      keepalive: true, // dokończ request nawet przy przejściu na inną stronę
    }).catch(function (err) {
      console.warn("[courses-hub] nie udało się wysłać postępu:", err);
    });
  }

  window.markCourseCompleted = function (courseSlug) {
    return sendProgress("completed", courseSlug);
  };

  // Nie cofaj ukończonego kursu do "w trakcie" przy ponownym wejściu.
  function markInProgress() {
    fetch("/api/courses")
      .then(function (res) { return res.ok ? res.json() : []; })
      .then(function (courses) {
        var course = courses.find(function (c) { return c.slug === slug; });
        if (!course || course.status !== "completed") sendProgress("in_progress");
      })
      .catch(function () { sendProgress("in_progress"); });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", markInProgress);
  } else {
    markInProgress();
  }
})();
