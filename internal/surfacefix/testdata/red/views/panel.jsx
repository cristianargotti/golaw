import { useState } from "react";
import { Chart } from "./charts.js";

export function Panel({ rows }) {
  return (
    <section>
      <div role="status">{rows.length} rows</div>
      <ul>
        {rows.map((row, index) => (
          <li key={index}>{row.name}</li>
        ))}
      </ul>
      <Chart rows={rows} />
    </section>
  );
}
