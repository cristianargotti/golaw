export function loadCharts() {
  const script = document.createElement("script");
  script.src = "https://cdn.example.com/charts/4.4.1/charts.js";
  document.head.append(script);
}
