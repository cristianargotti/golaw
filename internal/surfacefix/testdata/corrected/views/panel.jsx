import { Chart } from "./charts.js";

export function Panel({ rows }) {
  return (
    <section>
      <output>{rows.length} rows</output>
      <ul>
        {rows.map((row) => (
          <li key={row.id}>{row.name}</li>
        ))}
      </ul>
      <Chart rows={rows} />
    </section>
  );
}
