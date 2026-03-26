/**
 * Hide the individual spec pages from the Development nav sidebar unless the
 * user is currently browsing within the specs section.  The "Specs" section
 * header itself remains visible at all times so the user can navigate to it.
 *
 * MkDocs Material exposes a document$ observable that fires on every page
 * load and every instant-navigation transition, making this URL-based check
 * reliable regardless of how active classes are propagated internally.
 */
document$.subscribe(function () {
  var inSpecs = window.location.pathname.indexOf('/specs/') !== -1;

  // Find any link that points into the specs section, then walk up to its
  // nearest .md-nav__item--nested ancestor (the Specs section item).
  var specsLink = document.querySelector('.md-nav a[href*="specs/"]');
  if (!specsLink) return;

  var specsItem = specsLink.closest('.md-nav__item--nested');
  if (!specsItem) return;

  // Hide/show only the child nav (the individual spec page list), not the
  // specs section item itself, so the header link remains clickable.
  var childNav = specsItem.querySelector(':scope > .md-nav');
  if (!childNav) return;

  childNav.style.display = inSpecs ? '' : 'none';
});
