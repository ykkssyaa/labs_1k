import { h, iconUrl, render } from '../../uikit/dom.js';
import { errorPlaceholder, loader, ratingBadge } from '../../uikit/widgets.js';
import { placeTypeIcon } from '../../domain/place.js';

// последнее положение карты, чтобы при возврате на вкладку не сбрасывалось
let savedView = null;

/** Карта всех мест (Leaflet + OpenStreetMap). По маркеру — карточка и переход к месту. */
export function mapScreen(app) {
  const { l10n, router } = app;
  const mapEl = h('div', { class: 'map', 'data-testid': 'map' });
  const overlay = h('div', { class: 'map__overlay' }, loader());
  const el = h('div', { class: 'screen map-screen' }, mapEl, overlay);
  let map = null;

  function popup(place) {
    return h(
      'div',
      { class: 'map-popup' },
      h('b', {}, place.name),
      h('div', { class: 'map-popup__meta' }, l10n.t(`placeType.${place.type}`), ratingBadge(app, place)),
      h('button', { class: 'text-btn accent', onClick: () => router.go(`/map/${place.id}`, place) }, l10n.t('map.open')),
    );
  }

  async function load() {
    render(overlay, loader());
    overlay.hidden = false;
    const result = await app.placesRepository.getAllPlaces();
    if (!result.ok) {
      render(overlay, errorPlaceholder(app, result.error, load));
      return;
    }
    overlay.hidden = true;
    const markers = result.data.map((place) =>
      L.marker([place.lat, place.lon], {
        icon: L.divIcon({
          className: 'map-marker',
          html: `<span class="icon" style='--icon:${iconUrl(placeTypeIcon(place.type))}'></span>`,
          iconSize: [32, 32],
        }),
        title: place.name,
      }).bindPopup(() => popup(place)),
    );
    const group = L.featureGroup(markers).addTo(map);
    if (!savedView) map.fitBounds(group.getBounds(), { padding: [24, 24] });
  }

  function onShow() {
    // карту создаём, когда контейнер уже в документе и у него есть размер
    map = L.map(mapEl, { zoomControl: true });
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 18,
      attribution: '&copy; OpenStreetMap',
    }).addTo(map);
    if (savedView) map.setView(savedView.center, savedView.zoom);
    else map.setView([55.75, 37.62], 5);
    map.on('moveend', () => (savedView = { center: map.getCenter(), zoom: map.getZoom() }));
    load();
  }

  return { el, onShow, dispose: () => map?.remove() };
}
