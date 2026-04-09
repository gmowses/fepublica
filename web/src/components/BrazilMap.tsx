import { useQuery } from "@tanstack/react-query";
import { MapContainer, GeoJSON } from "react-leaflet";
import type { PathOptions, Layer } from "leaflet";
import type { Feature } from "geojson";
import { useMemo } from "react";

// Public, permissively-licensed Brazil states GeoJSON.
const BRAZIL_STATES_URL =
  "https://raw.githubusercontent.com/codeforgermany/click_that_hood/main/public/data/brazil-states.geojson";

export function BrazilMap({
  heatByUF,
}: {
  heatByUF?: Record<string, number>;
}) {
  const geoQ = useQuery({
    queryKey: ["brazil-states-geojson"],
    queryFn: () => fetch(BRAZIL_STATES_URL).then((r) => r.json()),
    staleTime: 24 * 60 * 60 * 1000,
  });

  const maxHeat = useMemo(() => {
    if (!heatByUF) return 0;
    return Math.max(0, ...Object.values(heatByUF));
  }, [heatByUF]);

  const style = (feature?: Feature): PathOptions => {
    const uf = (feature?.properties as any)?.sigla || "";
    const heat = heatByUF?.[uf] ?? 0;
    const t = maxHeat > 0 ? heat / maxHeat : 0;
    const fill = t > 0 ? `rgba(255, 209, 102, ${0.2 + 0.7 * t})` : "rgba(139, 152, 169, 0.14)";
    return {
      fillColor: fill,
      weight: 0.7,
      color: "#8b98a9",
      fillOpacity: 1,
      className: "fepublica-uf-path",
    };
  };

  const onEachFeature = (feature: Feature, layer: Layer) => {
    const props = feature.properties as any;
    const name = props?.name || props?.nome || "desconhecido";
    const uf = props?.sigla || "";
    const heat = heatByUF?.[uf] ?? 0;
    const tooltip = heat > 0 ? `${name} (${uf}) — ${heat} eventos` : `${name} (${uf})`;
    (layer as any).bindTooltip(tooltip, { sticky: true });
  };

  return (
    <div className="h-72 md:h-80 rounded-md overflow-hidden border border-ink/10">
      {geoQ.isLoading && (
        <div className="h-full flex items-center justify-center text-ink-dim text-sm">
          carregando mapa…
        </div>
      )}
      {geoQ.isError && (
        <div className="h-full flex items-center justify-center text-ink-dim text-sm">
          não foi possível carregar o GeoJSON do Brasil
        </div>
      )}
      {geoQ.data && (
        <MapContainer
          center={[-14.5, -53]}
          zoom={3}
          scrollWheelZoom={false}
          zoomControl={false}
          attributionControl={false}
          style={{ height: "100%", width: "100%" }}
        >
          <GeoJSON
            data={geoQ.data}
            style={style as any}
            onEachFeature={onEachFeature}
          />
        </MapContainer>
      )}
    </div>
  );
}
