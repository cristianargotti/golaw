export function count(rows) {
  const unused = rows.length;
  return rows.filter(Boolean).length;
}
