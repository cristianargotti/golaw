export function Blocks({ rows }) {
  return (
    <ul>
      {rows.map((row, index) => (
        <li key={index}>{row.name}</li>
      ))}
    </ul>
  );
}
