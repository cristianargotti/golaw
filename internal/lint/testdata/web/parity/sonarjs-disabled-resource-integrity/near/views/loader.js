export function loadCharts() {
  const script = document.createElement("script");
  script.src = "https://cdn.example.com/charts/4.4.1/charts.js";
  script.integrity = "sha384-oqVuAfXRKap7fdgcCY5uykM6R9GqQ8KuxyRx7HNQlGYl1kPzQho1wx4JwY8wC";
  script.crossOrigin = "anonymous";
  document.head.append(script);
}
